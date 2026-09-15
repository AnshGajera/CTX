package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/engine"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fsnotify/fsnotify"
)

// Watcher watches for file changes and re-extracts.
type Watcher struct {
	root     string
	engine   *engine.ExtractionEngine
	store    *versioning.ContextStore
	debounce time.Duration
	autoPush bool
}

// NewWatcher creates a watcher.
func NewWatcher(root string, eng *engine.ExtractionEngine, store *versioning.ContextStore, debounce time.Duration) *Watcher {
	if debounce <= 0 {
		debounce = 5 * time.Second
	}
	return &Watcher{root: root, engine: eng, store: store, debounce: debounce}
}

// SetAutoPush enables auto-commit on change.
func (w *Watcher) SetAutoPush(v bool) { w.autoPush = v }

var watchSkip = map[string]bool{
	"node_modules": true, ".git": true, ".ctx": true, "dist": true,
	"build": true, ".next": true, "__pycache__": true, "target": true,
	"coverage": true, ".turbo": true, ".cache": true, "vendor": true,
}

func shouldSkipWatch(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if watchSkip[p] {
			return true
		}
	}
	return false
}

// Watch runs until ctx cancelled.
func (w *Watcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// subscribe dirs recursively
	_ = filepath.Walk(w.root, func(p string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if p != w.root && watchSkip[info.Name()] {
			return filepath.SkipDir
		}
		_ = watcher.Add(p)
		return nil
	})

	fmt.Println("Watching for changes... (Ctrl+C to stop)")
	timer := time.NewTimer(w.debounce)
	if !timer.Stop() {
		<-timer.C
	}
	var pending bool

	var mu sync.Mutex
	var extracting bool
	var extractPending bool

	extractOnce := func() {
		fmt.Println("Change detected, re-extracting...")
		newCtx, err := w.engine.Extract()
		if err != nil {
			fmt.Printf("re-extract failed: %v\n", err)
			return
		}
		ctxDir := filepath.Join(w.root, ".ctx")
		if err := newCtx.Save(ctxDir); err != nil {
			fmt.Printf("save failed: %v\n", err)
			return
		}
		fmt.Printf("Re-extracted: %d endpoints, %d models\n",
			countEndpoints(newCtx), countModels(newCtx))
		if w.autoPush {
			if _, deduped, err := w.store.Commit(newCtx, "watch: auto-update"); err != nil {
				fmt.Printf("auto-commit failed: %v\n", err)
			} else if deduped {
				fmt.Println("No changes — snapshot not duplicated")
			} else {
				fmt.Println("Auto-committed snapshot")
			}
		}
	}

	runExtract := func() {
		for {
			extractOnce()
			mu.Lock()
			if !extractPending {
				extracting = false
				mu.Unlock()
				return
			}
			extractPending = false
			mu.Unlock()
		}
	}

	triggerExtract := func() {
		mu.Lock()
		if extracting {
			extractPending = true
			mu.Unlock()
			return
		}
		extracting = true
		mu.Unlock()
		go runExtract()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if shouldSkipWatch(ev.Name) {
				continue
			}
			// newly created dirs: add to watch
			if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
				_ = watcher.Add(ev.Name)
				continue
			}
			pending = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.debounce)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("watch error: %v\n", err)
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			triggerExtract()
		}
	}
}

func countEndpoints(c *projctx.ProjectContext) int {
	if c.APIs == nil {
		return 0
	}
	return len(c.APIs.Endpoints)
}

func countModels(c *projctx.ProjectContext) int {
	if c.Database == nil {
		return 0
	}
	return len(c.Database.Models)
}
