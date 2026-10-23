package detector

import (
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

type pythonChecker struct{}

func (c *pythonChecker) Name() string { return "python" }

func (c *pythonChecker) Check(root string) (*CheckResult, error) {
	manifests := findManifestFiles(root, map[string]bool{"requirements.txt": true, "pyproject.toml": true, "pipfile": true, "setup.py": true})
	if len(manifests) == 0 {
		return nil, nil
	}
	res := &CheckResult{Confidence: 0.9, ProjectType: "api_service"}
	pm := "pip"
	for _, manifest := range manifests {
		base := strings.ToLower(filepath.Base(manifest))
		data, _ := os.ReadFile(manifest)
		s := strings.ToLower(string(data))
		switch base {
		case "pipfile":
			pm = "pipenv"
		case "pyproject.toml":
			if strings.Contains(s, "poetry") {
				pm = "poetry"
			} else if strings.Contains(s, "conda") {
				pm = "conda"
			}
		}
		if strings.Contains(s, "fastapi") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "FastAPI", Type: "backend"})
		}
		if strings.Contains(s, "django") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Django", Type: "backend"})
		}
		if strings.Contains(s, "flask") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Flask", Type: "backend"})
		}
		if strings.Contains(s, "sqlalchemy") {
			res.DatabaseTypes = append(res.DatabaseTypes, "sql")
		}
		if strings.Contains(s, "pymongo") || strings.Contains(s, "mongo") {
			res.DatabaseTypes = append(res.DatabaseTypes, "mongodb")
		}
		if strings.Contains(s, "psycopg2") || strings.Contains(s, "psycopg") {
			res.DatabaseTypes = append(res.DatabaseTypes, "postgresql")
		}
	}
	res.PackageManager = pm
	res.Languages = []projctx.Language{{Name: "Python", Percentage: 85}}
	return res, nil
}
