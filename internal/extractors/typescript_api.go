package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// TypeScriptAPIExtractor extracts Express/Fastify/Hono style routes.
type TypeScriptAPIExtractor struct{ Base }

func NewTypeScriptAPI(root string) *TypeScriptAPIExtractor {
	return &TypeScriptAPIExtractor{Base: NewBase(root)}
}

func (e *TypeScriptAPIExtractor) Name() string { return "typescript-api" }

var (
	reExpressRoute = regexp.MustCompile("(?:app|router|api|server|r)\\.(get|post|put|patch|delete|all)\\(\\s*['\"`]([^'\"`]+)['\"`]")
	reChainedRoute = regexp.MustCompile("\\.route\\(\\s*['\"`]([^'\"`]+)['\"`]\\s*\\)\\.(get|post|put|patch|delete)")
	reHandlerArg   = regexp.MustCompile("(?:app|router|api|server|r)\\.(?:get|post|put|patch|delete|all)\\(\\s*['\"`][^'\"`]+['\"`]\\s*,([^)]*)\\)")
	reAuthMw       = regexp.MustCompile(`(?i)\b(auth|authenticate|requireAuth|isAuthenticated|protect|guard|verifyToken|checkAuth)\b`)
	reValidateMw   = regexp.MustCompile(`(?i)\b(validate|validator|validation|schema)\b`)
	rePathParam    = regexp.MustCompile(`:([A-Za-z0-9_]+)|\[([A-Za-z0-9_]+)\]`)
)

func (e *TypeScriptAPIExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		base := filepath.Base(path)
		if !(strings.HasSuffix(base, ".ts") || strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".tsx") || strings.HasSuffix(base, ".jsx") || strings.HasSuffix(base, ".mjs")) {
			return nil
		}
		if strings.Contains(rel, "node_modules") || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts") {
			return nil
		}
		if info.Size() > 1_000_000 {
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
			// A line may hold several route registrations; match all.
			for _, m := range reExpressRoute.FindAllStringSubmatch(line, -1) {
				if len(m) != 3 {
					continue
				}
				method := strings.ToUpper(m[1])
				routePath := m[2]
				ep := projctx.APIEndpoint{
					Method: method, Path: routePath,
					File: filepath.ToSlash(rel), Line: lineNo,
					Handler:      guessHandler(line),
					AuthRequired: reAuthMw.MatchString(line),
				}
				if reValidateMw.MatchString(line) {
					ep.Middleware = append(ep.Middleware, "validation")
				}
				if reAuthMw.MatchString(line) {
					ep.Middleware = append(ep.Middleware, "auth")
				}
				ep.Parameters = pathParams(routePath)
				endpoints = append(endpoints, ep)
			}
			for _, m := range reChainedRoute.FindAllStringSubmatch(line, -1) {
				if len(m) != 3 {
					continue
				}
				ep := projctx.APIEndpoint{
					Method: strings.ToUpper(m[2]), Path: m[1],
					File: filepath.ToSlash(rel), Line: lineNo,
					Handler: guessHandler(line),
				}
				ep.Parameters = pathParams(m[1])
				endpoints = append(endpoints, ep)
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

func guessHandler(line string) string {
	// last identifier before closing paren
	m := reHandlerArg.FindStringSubmatch(line)
	if len(m) < 2 {
		return ""
	}
	args := strings.Split(m[1], ",")
	last := strings.TrimSpace(args[len(args)-1])
	last = strings.Trim(last, " )};")
	if idx := strings.Index(last, "=>"); idx >= 0 {
		return "inline"
	}
	if last == "" {
		return ""
	}
	parts := strings.Fields(last)
	return parts[len(parts)-1]
}

func pathParams(route string) []projctx.APIParam {
	var out []projctx.APIParam
	for _, m := range rePathParam.FindAllStringSubmatch(route, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		out = append(out, projctx.APIParam{Name: name, In: "path", Type: "string", Required: true})
	}
	return out
}
