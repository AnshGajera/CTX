package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// PythonAPIExtractor extracts Flask/FastAPI/Django routes.
type PythonAPIExtractor struct{ Base }

func NewPythonAPI(root string) *PythonAPIExtractor {
	return &PythonAPIExtractor{Base: NewBase(root)}
}

func (e *PythonAPIExtractor) Name() string { return "python-api" }

var (
	reFlaskRoute   = regexp.MustCompile(`@\w+\.route\(\s*['"]([^'"]+)['"](?:[^)]*methods\s*=\s*\[([^\]]+)\])?`)
	reFastAPIRoute = regexp.MustCompile(`@\w+\.(get|post|put|patch|delete)\(\s*['"]([^'"]+)['"]`)
	reDjangoPath   = regexp.MustCompile(`path\(\s*['"]([^'"]+)['"]\s*,\s*([A-Za-z0-9_.]+)`)
	reDefLine      = regexp.MustCompile(`^\s*def\s+(\w+)\s*\(`)
)

// isTestPython skips test files/dirs so fixtures don't pollute routes.
func isTestPython(rel string) bool {
	base := filepath.Base(rel)
	slash := filepathToSlash(rel)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") ||
		strings.Contains(slash, "/tests/") || strings.Contains(slash, "/test/") ||
		strings.HasPrefix(slash, "tests/") || strings.HasPrefix(slash, "test/")
}

func (e *PythonAPIExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".py") || isTestPython(rel) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 128*1024), 2*1024*1024)
		lineNo := 0
		pendingHandler := ""
		pendingLines := []struct {
			method, route string
			line          int
		}{}
		flush := func(handler string) {
			for _, p := range pendingLines {
				endpoints = append(endpoints, projctx.APIEndpoint{Method: p.method, Path: p.route, Handler: handler, File: filepath.ToSlash(rel), Line: p.line, Parameters: pathParams(p.route)})
			}
			pendingLines = nil
		}
		for sc.Scan() {
			lineNo++
			line := sc.Text()
			if m := reFastAPIRoute.FindStringSubmatch(line); len(m) == 3 {
				pendingLines = append(pendingLines, struct {
					method, route string
					line          int
				}{strings.ToUpper(m[1]), m[2], lineNo})
				continue
			}
			if m := reFlaskRoute.FindStringSubmatch(line); len(m) >= 2 {
				method := "GET"
				if len(m) >= 3 && m[2] != "" {
					method = strings.ToUpper(strings.Trim(m[2], "'\" "))
					if idx := strings.Index(method, ","); idx > 0 {
						method = strings.TrimSpace(method[:idx])
					}
					method = strings.Trim(method, "'\"")
				}
				pendingLines = append(pendingLines, struct {
					method, route string
					line          int
				}{method, m[1], lineNo})
				continue
			}
			if m := reDjangoPath.FindStringSubmatch(line); len(m) == 3 {
				endpoints = append(endpoints, projctx.APIEndpoint{Method: "ALL", Path: "/" + m[1], Handler: m[2], File: filepath.ToSlash(rel), Line: lineNo, Parameters: pathParams(m[1])})
				continue
			}
			if m := reDefLine.FindStringSubmatch(line); len(m) == 2 {
				pendingHandler = m[1]
				if len(pendingLines) > 0 {
					flush(pendingHandler)
				}
				_ = pendingHandler
				continue
			}
		}
		// decorators at EOF without def seen
		for _, p := range pendingLines {
			endpoints = append(endpoints, projctx.APIEndpoint{Method: p.method, Path: p.route, Handler: "", File: filepath.ToSlash(rel), Line: p.line, Parameters: pathParams(p.route)})
		}
		return nil
	})
	if len(endpoints) == 0 {
		return nil
	}
	if ctx.APIs == nil {
		ctx.APIs = &projctx.APIContext{}
	}
	ctx.APIs.Endpoints = append(ctx.APIs.Endpoints, endpoints...)
	return nil
}
