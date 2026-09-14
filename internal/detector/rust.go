package detector

import (
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

type rustChecker struct{}

func (c *rustChecker) Name() string { return "rust" }

func (c *rustChecker) Check(root string) (*CheckResult, error) {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return nil, nil
	}
	s := string(data)
	res := &CheckResult{Confidence: 0.9, PackageManager: "cargo", ProjectType: "api_service"}
	for _, fw := range []string{"actix-web", "axum", "rocket", "warp"} {
		if strings.Contains(s, fw) {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: fw, Type: "backend"})
		}
	}
	if strings.Contains(s, "diesel") {
		res.DatabaseTypes = append(res.DatabaseTypes, "sql")
		res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "Diesel", Type: "database"})
	}
	if strings.Contains(s, "sqlx") {
		res.DatabaseTypes = append(res.DatabaseTypes, "sql")
	}
	res.Languages = []projctx.Language{{Name: "Rust", Percentage: 90}}
	return res, nil
}
