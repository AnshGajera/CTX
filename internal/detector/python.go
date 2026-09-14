package detector

import (
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

type pythonChecker struct{}

func (c *pythonChecker) Name() string { return "python" }

func (c *pythonChecker) Check(root string) (*CheckResult, error) {
	hasReq := fileExists(root, "requirements.txt")
	hasPyproject := fileExists(root, "pyproject.toml")
	hasPipfile := fileExists(root, "Pipfile")
	hasSetup := fileExists(root, "setup.py")
	if !hasReq && !hasPyproject && !hasPipfile && !hasSetup {
		return nil, nil
	}
	res := &CheckResult{Confidence: 0.9, ProjectType: "api_service"}
	pm := "pip"
	switch {
	case hasPipfile:
		pm = "pipenv"
	case hasPyproject:
		data, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		s := string(data)
		if strings.Contains(s, "poetry") {
			pm = "poetry"
		} else if strings.Contains(s, "conda") {
			pm = "conda"
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
	if hasReq {
		data, _ := os.ReadFile(filepath.Join(root, "requirements.txt"))
		s := strings.ToLower(string(data))
		if strings.Contains(s, "fastapi") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "FastAPI", Type: "backend"})
		}
		if strings.Contains(s, "django") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Django", Type: "backend"})
		}
		if strings.Contains(s, "flask") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Flask", Type: "backend"})
		}
		if strings.Contains(s, "starlette") {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Starlette", Type: "backend"})
		}
		if strings.Contains(s, "sqlalchemy") {
			res.DatabaseTypes = append(res.DatabaseTypes, "sql")
		}
		if strings.Contains(s, "pymongo") {
			res.DatabaseTypes = append(res.DatabaseTypes, "mongodb")
		}
	}
	res.PackageManager = pm
	res.Languages = []projctx.Language{{Name: "Python", Percentage: 85}}
	return res, nil
}
