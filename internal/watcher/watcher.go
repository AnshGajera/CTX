package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	projctx "github.com/ctxdev/ctx/internal/context"
	"github.com/ctxdev/ctx/internal/engine"
	"github.com/ctxdev/ctx/internal/versioning"
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
			fmt.Println("Change detected, re-extracting...")
			newCtx, err := w.engine.Extract()
			if err != nil {
				fmt.Printf("re-extract failed: %v\n", err)
				continue
			}
			ctxDir := filepath.Join(w.root, ".ctx")
			if err := newCtx.Save(ctxDir); err != nil {
				fmt.Printf("save failed: %v\n", err)
				continue
			}
			fmt.Printf("Re-extracted: %d endpoints, %d models\n",
				countEndpoints(newCtx), countModels(newCtx))
			if w.autoPush {
				if _, err := w.store.Commit(newCtx, "watch: auto-update"); err != nil {
					fmt.Printf("auto-commit failed: %v\n", err)
				} else {
					fmt.Println("Auto-committed snapshot")
				}
			}
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
