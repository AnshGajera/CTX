package extractors

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// FileStructureExtractor builds the directory tree + key files + conventions.
type FileStructureExtractor struct{ Base }

func NewFileStructure(root string) *FileStructureExtractor {
	return &FileStructureExtractor{Base: NewBase(root)}
}

func (e *FileStructureExtractor) Name() string { return "file-structure" }

var dirPurpose = map[string]string{
	"src": "source code", "lib": "libraries", "app": "app router / main app",
	"pages": "pages / routes", "api": "api endpoints", "components": "ui components",
	"hooks": "react hooks", "utils": "utilities", "helpers": "helpers",
	"services": "service layer", "models": "data models", "controllers": "controllers",
	"middleware": "middleware", "routes": "route definitions", "config": "configuration",
	"types": "type definitions", "interfaces": "interfaces", "constants": "constants",
	"store": "state store", "context": "react context", "providers": "providers",
	"layouts": "layouts", "styles": "styles", "assets": "static assets",
	"public": "public assets", "static": "static files", "tests": "tests",
	"__tests__": "tests", "test": "tests", "spec": "specs", "e2e": "end-to-end tests",
	"cypress": "cypress tests", "playwright": "playwright tests", "migrations": "db migrations",
	"seeds": "db seeds", "prisma": "prisma schema", "scripts": "scripts",
	"docs": "documentation", "deploy": "deployment", "infra": "infrastructure",
	"terraform": "terraform", "k8s": "kubernetes", "cmd": "go entrypoints",
	"pkg": "go packages", "internal": "private go packages", "handlers": "http handlers",
	"repositories": "repositories", "domain": "domain layer", "entities": "entities",
	"usecases": "use cases", "features": "feature modules", "modules": "modules",
	"shared": "shared code", "common": "common code", "core": "core logic",
	"auth": "authentication", "i18n": "internationalization", "locales": "locales",
}

func inferDirectoryPurpose(name string) string {
	if p, ok := dirPurpose[strings.ToLower(name)]; ok {
		return p
	}
	return ""
}

func (e *FileStructureExtractor) Extract(ctx *projctx.ProjectContext) error {
	rootNode := &projctx.DirectoryNode{Name: filepath.Base(e.Root), Purpose: "project root"}
	nodeMap := map[string]*projctx.DirectoryNode{".": rootNode}

	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		dir := filepath.Dir(rel)
		// ensure chain up to 4 levels
		parts := strings.Split(filepath.ToSlash(dir), "/")
		if len(parts) > 4 {
			parts = parts[:4]
		}
		parent := rootNode
		acc := ""
		for i, part := range parts {
			if part == "." || part == "" {
				continue
			}
			if acc == "" {
				acc = part
			} else {
				acc = acc + "/" + part
			}
			if n, ok := nodeMap[acc]; ok {
				parent = n
			} else {
				n := &projctx.DirectoryNode{Name: part, Purpose: inferDirectoryPurpose(part)}
				nodeMap[acc] = n
				parent.Children = append(parent.Children, n)
				parent = n
			}
			_ = i
		}
		parent.FileCount++
		return nil
	})

	keyFiles := e.identifyKeyFiles()
	conventions := e.detectConventions()

	ctx.FileStructure = &projctx.FileStructureContext{
		Tree:        rootNode,
		KeyFiles:    keyFiles,
		Conventions: conventions,
	}
	return nil
}

var keyFileCandidates = []struct {
	Pattern string
	Purpose string
}{
	{"package.json", "node manifest"}, {"tsconfig.json", "typescript config"},
	{"go.mod", "go modules"}, {"Cargo.toml", "rust manifest"},
	{"requirements.txt", "python deps"}, {"pyproject.toml", "python project"},
	{"Dockerfile", "container build"}, {"docker-compose.yml", "compose services"},
	{"docker-compose.yaml", "compose services"}, {"Makefile", "build tasks"},
	{"README.md", "readme"}, {"CONTRIBUTING.md", "contributing"},
	{"ARCHITECTURE.md", "architecture doc"}, {".env.example", "env template"},
	{"middleware.ts", "edge middleware"},
}

var keyFileGlobs = []string{"*.config.ts", "*.config.js", ".eslintrc*", ".prettierrc*", "tailwind.config.*", "vite.config.*", "next.config.*", "jest.config.*", "vitest.config.*"}

func (e *FileStructureExtractor) identifyKeyFiles() []projctx.KeyFile {
	var out []projctx.KeyFile
	for _, c := range keyFileCandidates {
		p := filepath.Join(e.Root, c.Pattern)
		if _, err := os.Stat(p); err == nil {
			out = append(out, projctx.KeyFile{Path: c.Pattern, Purpose: c.Purpose, Importance: "high"})
			continue
		}
		if strings.Contains(c.Pattern, "workflows") {
			continue
		}
	}
	if _, err := os.Stat(filepath.Join(e.Root, ".github", "workflows")); err == nil {
		out = append(out, projctx.KeyFile{Path: ".github/workflows", Purpose: "ci pipelines", Importance: "medium"})
	}
	for _, g := range keyFileGlobs {
		matches, _ := filepath.Glob(filepath.Join(e.Root, g))
		for _, m := range matches {
			rel, _ := filepath.Rel(e.Root, m)
			out = append(out, projctx.KeyFile{Path: filepath.ToSlash(rel), Purpose: "config file", Importance: "medium"})
		}
	}
	return out
}

var conventionPatterns = []struct {
	Re   *regexp.Regexp
	Name string
	Desc string
}{
	{regexp.MustCompile(`\.action\.ts$`), "*.action.ts", "server actions"},
	{regexp.MustCompile(`\.service\.ts$`), "*.service.ts", "service layer"},
	{regexp.MustCompile(`^use.+\.ts$`), "use*.ts", "react hooks"},
	{regexp.MustCompile(`\.schema\.ts$`), "*.schema.ts", "validation schemas"},
	{regexp.MustCompile(`\.controller\.ts$`), "*.controller.ts", "controllers"},
	{regexp.MustCompile(`\.repository\.ts$`), "*.repository.ts", "repositories"},
	{regexp.MustCompile(`\.test\.ts$`), "*.test.ts", "unit tests"},
	{regexp.MustCompile(`\.spec\.ts$`), "*.spec.ts", "specs"},
	{regexp.MustCompile(`\.module\.ts$`), "*.module.ts", "nestjs modules"},
	{regexp.MustCompile(`\.dto\.ts$`), "*.dto.ts", "data transfer objects"},
	{regexp.MustCompile(`\.entity\.ts$`), "*.entity.ts", "entities"},
	{regexp.MustCompile(`\.guard\.ts$`), "*.guard.ts", "route guards"},
	{regexp.MustCompile(`\.pipe\.ts$`), "*.pipe.ts", "validation pipes"},
}

func (e *FileStructureExtractor) detectConventions() []projctx.NamingConvention {
	counts := map[string]int{}
	examples := map[string]string{}
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		base := filepath.Base(rel)
		for _, c := range conventionPatterns {
			if c.Re.MatchString(base) {
				counts[c.Name]++
				if _, ok := examples[c.Name]; !ok {
					examples[c.Name] = filepath.ToSlash(rel)
				}
			}
		}
		return nil
	})
	var out []projctx.NamingConvention
	for _, c := range conventionPatterns {
		if counts[c.Name] > 0 {
			out = append(out, projctx.NamingConvention{Pattern: c.Name, Example: examples[c.Name], Description: c.Desc})
		}
	}
	return out
}
