package detector

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

// CheckResult is the output of a single checker.
type CheckResult struct {
	Languages      []projctx.Language
	Frameworks     []projctx.Framework
	PackageManager string
	ProjectType    string
	IsMonorepo     bool
	DatabaseTypes  []string
	ContainerType  string
	Confidence     float64
}

type checker interface {
	Name() string
	Check(root string) (*CheckResult, error)
}

// Detector runs all checkers against a root.
type Detector struct {
	root     string
	checkers []checker
}

// New creates a Detector with all checkers registered.
func New(root string) *Detector {
	return &Detector{
		root: root,
		checkers: []checker{
			&nodeChecker{},
			&goChecker{},
			&pythonChecker{},
			&rustChecker{},
			&javaChecker{},
			&dockerChecker{},
		},
	}
}

// Detect runs all checkers and merges results.
func (d *Detector) Detect() (*projctx.ProjectProfile, error) {
	profile := &projctx.ProjectProfile{}
	fwSeen := map[string]bool{}
	dbSeen := map[string]bool{}
	for _, c := range d.checkers {
		res, err := c.Check(d.root)
		if err != nil || res == nil {
			continue
		}
		profile.Languages = append(profile.Languages, res.Languages...)
		for _, f := range res.Frameworks {
			key := strings.ToLower(f.Name)
			if !fwSeen[key] {
				fwSeen[key] = true
				profile.Frameworks = append(profile.Frameworks, f)
			}
		}
		if res.PackageManager != "" && profile.PackageManager == "" {
			profile.PackageManager = res.PackageManager
		}
		if res.ProjectType != "" && profile.ProjectType == "" {
			profile.ProjectType = res.ProjectType
		}
		if res.IsMonorepo {
			profile.IsMonorepo = true
		}
		for _, db := range res.DatabaseTypes {
			if !dbSeen[db] {
				dbSeen[db] = true
				profile.DatabaseTypes = append(profile.DatabaseTypes, db)
			}
		}
		if res.ContainerType != "" {
			profile.ContainerType = res.ContainerType
		}
	}
	// Language distribution from file extensions always runs to ground truth.
	dist := d.analyzeLanguageDistribution()
	if len(dist) > 0 {
		profile.Languages = mergeLanguages(profile.Languages, dist)
	}
	profile.EntryPoints = d.findEntryPoints()
	if profile.ProjectType == "" {
		profile.ProjectType = inferProjectType(profile)
	}
	return profile, nil
}

func mergeLanguages(a, b []projctx.Language) []projctx.Language {
	byName := map[string]projctx.Language{}
	for _, l := range a {
		byName[strings.ToLower(l.Name)] = l
	}
	for _, l := range b {
		k := strings.ToLower(l.Name)
		if existing, ok := byName[k]; ok {
			if existing.Version == "" {
				existing.Version = l.Version
			}
			if existing.Percentage == 0 {
				existing.Percentage = l.Percentage
			}
			byName[k] = existing
		} else {
			byName[k] = l
		}
	}
	out := make([]projctx.Language, 0, len(byName))
	for _, v := range byName {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Percentage > out[j].Percentage })
	return out
}

var extToLang = map[string]string{
	".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript", ".jsx": "JavaScript",
	".mjs": "JavaScript", ".cjs": "JavaScript", ".go": "Go", ".py": "Python",
	".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".rb": "Ruby", ".php": "PHP",
	".cs": "C#", ".cpp": "C++", ".c": "C", ".h": "C++", ".swift": "Swift",
	".tf": "HCL", ".sql": "SQL", ".sh": "Shell", ".vue": "Vue", ".svelte": "Svelte",
}

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "__pycache__": true, ".venv": true,
	"venv": true, ".idea": true, ".vscode": true, "coverage": true,
	".ctx": true, ".turbo": true, ".cache": true, "target": true,
}

func (d *Detector) analyzeLanguageDistribution() []projctx.Language {
	counts := map[string]int{}
	total := 0
	_ = filepath.Walk(d.root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		lang, ok := extToLang[strings.ToLower(filepath.Ext(p))]
		if !ok {
			return nil
		}
		counts[lang]++
		total++
		return nil
	})
	if total == 0 {
		return nil
	}
	out := make([]projctx.Language, 0, len(counts))
	for name, c := range counts {
		out = append(out, projctx.Language{
			Name:       name,
			Percentage: float64(c) * 100.0 / float64(total),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Percentage > out[j].Percentage })
	return out
}

var entryCandidates = []string{
	"main.go", "cmd/main.go", "index.ts", "src/index.ts", "src/main.ts",
	"index.js", "src/index.js", "app.py", "main.py", "src/main.py",
	"src/main.rs", "src/lib.rs", "pom.xml", "build.gradle", "manage.py",
	"next.config.js", "next.config.mjs", "vite.config.ts", "package.json",
}

func (d *Detector) findEntryPoints() []string {
	var out []string
	for _, c := range entryCandidates {
		if _, err := os.Stat(filepath.Join(d.root, c)); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func inferProjectType(p *projctx.ProjectProfile) string {
	for _, f := range p.Frameworks {
		n := strings.ToLower(f.Name)
		switch n {
		case "next", "nuxt", "remix", "astro", "react", "vue", "angular", "svelte":
			return "web_application"
		case "express", "fastify", "nestjs", "hono", "gin", "echo", "fiber", "chi", "flask", "fastapi", "django", "actix-web", "axum", "rocket", "spring-boot":
			return "api_service"
		case "cobra", "urfave/cli":
			return "cli_tool"
		case "react-native", "electron":
			return "desktop_mobile"
		}
	}
	for _, l := range p.Languages {
		if strings.EqualFold(l.Name, "Go") && len(p.Frameworks) == 0 {
			return "cli_tool"
		}
	}
	if p.IsMonorepo {
		return "monorepo"
	}
	return "library"
}

func fileExists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
