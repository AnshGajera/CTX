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

func NewPythonAPI(root string) *PythonAPIExtractor { return &PythonAPIExtractor{Base: NewBase(root)} }
func (e *PythonAPIExtractor) Name() string         { return "python-api" }

var (
	reFlaskRoute   = regexp.MustCompile(`@\w+\.route\(\s*['"]([^'"]+)['"](?:[^)]*methods\s*=\s*\[([^\]]+)\])?`)
	reFastAPIRoute = regexp.MustCompile(`@\w+\.(get|post|put|patch|delete)\(\s*['"]([^'"]+)['"]`)
	reDjangoPath   = regexp.MustCompile(`path\(\s*['"]([^'"]+)['"]\s*,\s*([A-Za-z0-9_.]+)`)
	reDefLine      = regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)\s*\(`)
)

func isTestPython(rel string) bool {
	base := filepath.Base(rel)
	slash := filepathToSlash(rel)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") ||
		strings.Contains(slash, "/tests/") || strings.Contains(slash, "/test/") ||
		strings.HasPrefix(slash, "tests/") || strings.HasPrefix(slash, "test/")
}

type pythonRoute struct {
	method, route, description string
	line                       int
	auth                       bool
}

func (e *PythonAPIExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".py") || isTestPython(rel) || info.Size() > 1_000_000 {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 128*1024), 2*1024*1024)
		var lines []string
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		var pending []pythonRoute
		for index, line := range lines {
			lineNo := index + 1
			for _, m := range reFastAPIRoute.FindAllStringSubmatch(line, -1) {
				pending = append(pending, pythonRoute{method: strings.ToUpper(m[1]), route: m[2], line: lineNo, auth: isAuthLine(line)})
			}
			for _, m := range reFlaskRoute.FindAllStringSubmatch(line, -1) {
				method := "GET"
				if len(m) > 2 && m[2] != "" {
					method = strings.ToUpper(strings.Trim(strings.Split(m[2], ",")[0], "'\" "))
				}
				pending = append(pending, pythonRoute{method: method, route: m[1], line: lineNo, auth: isAuthLine(line)})
			}
			for _, m := range reDjangoPath.FindAllStringSubmatch(line, -1) {
				endpoints = append(endpoints, projctx.APIEndpoint{Method: "ALL", Path: "/" + m[1], Handler: m[2], File: filepath.ToSlash(rel), Line: lineNo, Parameters: pathParams(m[1])})
			}
			if m := reDefLine.FindStringSubmatch(line); len(m) == 2 && len(pending) > 0 {
				description := pythonDocstring(lines, index)
				for _, route := range pending {
					if description == "" {
						description = route.description
					}
					endpoints = append(endpoints, projctx.APIEndpoint{
						Method: route.method, Path: route.route, Handler: m[1],
						File: filepath.ToSlash(rel), Line: route.line, Description: description,
						Parameters:   pathParams(route.route),
						AuthRequired: route.auth || isAuthLine(line) || strings.Contains(strings.ToLower(m[1]), "auth"),
					})
				}
				pending = nil
			}
		}
		for _, route := range pending {
			endpoints = append(endpoints, projctx.APIEndpoint{Method: route.method, Path: route.route, File: filepath.ToSlash(rel), Line: route.line, Parameters: pathParams(route.route), AuthRequired: route.auth})
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

func isAuthLine(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"depends(get_current_user", "depends(require_auth", "depends(auth", "requires_auth", "jwt", "current_user", "admin_only"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func pythonDocstring(lines []string, defIndex int) string {
	for i := defIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, "'''") {
			quote := line[:3]
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, quote), quote))
			if text != "" {
				return text
			}
			if i+1 < len(lines) {
				return strings.TrimSpace(lines[i+1])
			}
		}
		break
	}
	return ""
}
