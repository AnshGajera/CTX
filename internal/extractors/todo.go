package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// TODOExtractor collects TODO/FIXME/HACK/XXX/BUG comments.
type TODOExtractor struct{ Base }

func NewTODO(root string) *TODOExtractor {
	return &TODOExtractor{Base: NewBase(root)}
}

func (e *TODOExtractor) Name() string { return "todo" }

var reTodo = regexp.MustCompile(`(?:TODO|FIXME|HACK|XXX|BUG)[\s:]+(.+)$`)

func (e *TODOExtractor) Extract(ctx *projctx.ProjectContext) error {
	var todos []projctx.TODO
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if len(todos) >= 100 {
			return filepath.SkipDir
		}
		if !IsSourceFile(filepath.Base(path)) {
			// also scan .md? no, keep to source
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			if m := reTodo.FindStringSubmatch(sc.Text()); len(m) > 1 {
				todos = append(todos, projctx.TODO{Text: m[1], File: filepath.ToSlash(rel), Line: lineNo})
				if len(todos) >= 100 {
					break
				}
			}
		}
		return nil
	})
	if ctx.CurrentState == nil {
		ctx.CurrentState = &projctx.ProjectStateContext{}
	}
	ctx.CurrentState.TODOs = todos
	return nil
}
