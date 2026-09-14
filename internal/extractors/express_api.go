package extractors

import (
	projctx "github.com/AnshGajera/CTX/internal/context"
)

// ExpressAPIExtractor is an alias-style extractor for Express validation details.
// The main TS route extraction lives in typescript_api.go; this pass enriches
// middleware/auth detection for express-like projects.
type ExpressAPIExtractor struct{ Base }

func NewExpressAPI(root string) *ExpressAPIExtractor {
	return &ExpressAPIExtractor{Base: NewBase(root)}
}

func (e *ExpressAPIExtractor) Name() string { return "express-api" }

func (e *ExpressAPIExtractor) Extract(ctx *projctx.ProjectContext) error {
	// Enrichment: if APIs already extracted, mark auth type default.
	if ctx.APIs != nil && len(ctx.APIs.Endpoints) > 0 && ctx.APIs.AuthType == "" {
		hasAuth := false
		for _, ep := range ctx.APIs.Endpoints {
			if ep.AuthRequired {
				hasAuth = true
				break
			}
		}
		if hasAuth {
			ctx.APIs.AuthType = "middleware"
		}
	}
	return nil
}
