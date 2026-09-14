package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

type nodeChecker struct{}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      any               `json:"workspaces"`
	Scripts         map[string]string `json:"scripts"`
}

var nodeFrameworkMap = map[string]string{
	"next": "Next.js", "react": "React", "react-dom": "React", "vue": "Vue",
	"nuxt": "Nuxt", "@angular/core": "Angular", "svelte": "Svelte",
	"express": "Express", "fastify": "Fastify", "@nestjs/core": "NestJS",
	"hono": "Hono", "remix": "Remix", "astro": "Astro",
	"electron": "Electron", "react-native": "React Native",
}

var nodeDBMap = map[string]string{
	"prisma": "postgresql", "@prisma/client": "postgresql", "mongoose": "mongodb",
	"pg": "postgresql", "mysql2": "mysql", "redis": "redis", "ioredis": "redis",
	"typeorm": "sql", "drizzle-orm": "sql", "@supabase/supabase-js": "postgresql",
	"mongodb": "mongodb", "sqlite3": "sqlite", "better-sqlite3": "sqlite",
}

func (c *nodeChecker) Name() string { return "node" }

func (c *nodeChecker) Check(root string) (*CheckResult, error) {
	pjPath := filepath.Join(root, "package.json")
	data, err := os.ReadFile(pjPath)
	if err != nil {
		return nil, nil
	}
	var pj packageJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return &CheckResult{}, nil
	}
	res := &CheckResult{Confidence: 0.9, ProjectType: "web_application"}
	all := map[string]string{}
	for k, v := range pj.Dependencies {
		all[k] = v
	}
	for k, v := range pj.DevDependencies {
		all[k] = v
	}
	hasTS := false
	if _, ok := all["typescript"]; ok {
		hasTS = true
	}
	for dep, ver := range all {
		if fw, ok := nodeFrameworkMap[dep]; ok {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: fw, Version: ver, Type: "frontend"})
		}
		if db, ok := nodeDBMap[dep]; ok {
			res.DatabaseTypes = append(res.DatabaseTypes, db)
		}
		_ = ver
	}
	// test frameworks as frameworks of type testing
	for _, t := range []string{"jest", "vitest", "mocha", "cypress", "playwright", "@playwright/test"} {
		if v, ok := all[t]; ok {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: t, Version: v, Type: "testing"})
		}
	}
	if hasTS {
		res.Languages = append(res.Languages, projctx.Language{Name: "TypeScript", Percentage: 60})
		res.Languages = append(res.Languages, projctx.Language{Name: "JavaScript", Percentage: 30})
	} else {
		res.Languages = append(res.Languages, projctx.Language{Name: "JavaScript", Percentage: 80})
	}
	res.PackageManager = detectNodePM(root)
	if pj.Workspaces != nil || fileExists(root, "lerna.json") || fileExists(root, "pnpm-workspace.yaml") {
		res.IsMonorepo = true
	}
	if fileExists(root, "prisma/schema.prisma") {
		res.DatabaseTypes = append(res.DatabaseTypes, "postgresql")
	}
	// express-only projects are api services
	if _, ok := all["express"]; ok {
		if _, ok2 := all["next"]; !ok2 {
			if _, ok3 := all["react"]; !ok3 {
				res.ProjectType = "api_service"
			}
		}
	}
	_ = strings.TrimSpace("")
	return res, nil
}

func detectNodePM(root string) string {
	switch {
	case fileExists(root, "pnpm-lock.yaml"):
		return "pnpm"
	case fileExists(root, "yarn.lock"):
		return "yarn"
	case fileExists(root, "bun.lockb"):
		return "bun"
	case fileExists(root, "package-lock.json"):
		return "npm"
	default:
		return "npm"
	}
}
