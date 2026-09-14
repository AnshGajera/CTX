package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// GoAPIExtractor extracts Go HTTP handlers.
type GoAPIExtractor struct{ Base }

func NewGoAPI(root string) *GoAPIExtractor {
	return &GoAPIExtractor{Base: NewBase(root)}
}

func (e *GoAPIExtractor) Name() string { return "go-api" }

var (
	reHandleFunc = regexp.MustCompile(`http\.HandleFunc\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)`)
	reMuxHandle  = regexp.MustCompile(`\.HandleFunc\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)\s*\)(?:\.Methods\(\s*"([A-Z]+)"[^)]*\))?`)
	reChiRoute   = regexp.MustCompile(`\.(Get|Post|Put|Patch|Delete)\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)`)
	reGinRoute   = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)`)
	reEchoRoute  = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)`)
	reFiberRoute = regexp.MustCompile(`\.(Get|Post|Put|Patch|Delete)\(\s*"([^"]+)"\s*,\s*([A-Za-z0-9_.]+)`)
)

func (e *GoAPIExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
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
		for sc.Scan() {
			lineNo++
			line := sc.Text()
			if m := reMuxHandle.FindStringSubmatch(line); len(m) >= 3 {
				method := "ALL"
				if len(m) >= 4 && m[3] != "" {
					method = m[3]
				}
				endpoints = append(endpoints, projctx.APIEndpoint{Method: method, Path: m[1], Handler: m[2], File: filepath.ToSlash(rel), Line: lineNo, Parameters: pathParams(m[1])})
				continue
			}
			if m := reHandleFunc.FindStringSubmatch(line); len(m) == 3 {
				endpoints = append(endpoints, projctx.APIEndpoint{Method: "ALL", Path: m[1], Handler: m[2], File: filepath.ToSlash(rel), Line: lineNo, Parameters: pathParams(m[1])})
				continue
			}
			for _, re := range []*regexp.Regexp{reChiRoute, reGinRoute, reEchoRoute, reFiberRoute} {
				if m := re.FindStringSubmatch(line); len(m) == 4 {
					endpoints = append(endpoints, projctx.APIEndpoint{Method: strings.ToUpper(m[1]), Path: m[2], Handler: m[3], File: filepath.ToSlash(rel), Line: lineNo, Parameters: pathParams(m[2])})
					break
				}
			}
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
