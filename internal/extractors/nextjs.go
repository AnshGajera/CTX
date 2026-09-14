package extractors

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

// NextJSExtractor extracts App Router + Pages Router routes.
type NextJSExtractor struct{ Base }

func NewNextJS(root string) *NextJSExtractor {
	return &NextJSExtractor{Base: NewBase(root)}
}

func (e *NextJSExtractor) Name() string { return "nextjs" }

var reExportedMethod = regexp.MustCompile(`export\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE)\b`)

func (e *NextJSExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint
	appAPI := filepath.Join(e.Root, "app", "api")
	if _, err := os.Stat(appAPI); err == nil {
		_ = filepath.Walk(appAPI, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			if base != "route.ts" && base != "route.js" && base != "route.tsx" {
				return nil
			}
			rel, _ := filepath.Rel(e.Root, p)
			apiPath := appRouteToPath(rel)
			data, _ := os.ReadFile(p)
			methods := reExportedMethod.FindAllStringSubmatch(string(data), -1)
			if len(methods) == 0 {
				// default: file exists but no export match — assume GET
				methods = [][]string{{"", "GET"}}
			}
			for _, m := range methods {
				endpoints = append(endpoints, projctx.APIEndpoint{
					Method: m[1], Path: apiPath,
					File: filepath.ToSlash(rel), Handler: m[1],
					Parameters: pathParams(apiPath),
				})
			}
			return nil
		})
	}
	// Pages router: pages/api/**/*.ts|js
	pagesAPI := filepath.Join(e.Root, "pages", "api")
	if _, err := os.Stat(pagesAPI); err == nil {
		_ = filepath.Walk(pagesAPI, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(p)
			if ext != ".ts" && ext != ".js" && ext != ".tsx" {
				return nil
			}
			rel, _ := filepath.Rel(e.Root, p)
			apiPath := pagesRouteToPath(rel)
			endpoints = append(endpoints, projctx.APIEndpoint{
				Method: "ALL", Path: apiPath, File: filepath.ToSlash(rel),
				Handler: "handler", Parameters: pathParams(apiPath),
			})
			return nil
		})
	}
	if len(endpoints) == 0 {
		return nil
	}
	if ctx.APIs == nil {
		ctx.APIs = &projctx.APIContext{}
	}
	ctx.APIs.Endpoints = append(ctx.APIs.Endpoints, endpoints...)
	return nil
}

func appRouteToPath(rel string) string {
	// app/api/users/[id]/route.ts -> /api/users/:id
	s := filepath.ToSlash(rel)
	s = strings.TrimPrefix(s, "app")
	s = strings.TrimSuffix(s, "/route.ts")
	s = strings.TrimSuffix(s, "/route.js")
	s = strings.TrimSuffix(s, "/route.tsx")
	// convert [param] and [...param] to :param
	re := regexp.MustCompile(`\[\.\.\.([^\]]+)\]`)
	s = re.ReplaceAllString(s, ":$1")
	re2 := regexp.MustCompile(`\[([^\]]+)\]`)
	s = re2.ReplaceAllString(s, ":$1")
	if s == "" {
		s = "/"
	}
	return s
}

func pagesRouteToPath(rel string) string {
	s := filepath.ToSlash(rel)
	s = strings.TrimPrefix(s, "pages")
	ext := filepath.Ext(s)
	s = strings.TrimSuffix(s, ext)
	s = strings.TrimSuffix(s, "/index")
	re2 := regexp.MustCompile(`\[([^\]]+)\]`)
	s = re2.ReplaceAllString(s, ":$1")
	if s == "" {
		s = "/"
	}
	return s
}
