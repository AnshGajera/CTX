package detector

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

type goChecker struct{}

func (c *goChecker) Name() string { return "go" }

func (c *goChecker) Check(root string) (*CheckResult, error) {
	gomod := filepath.Join(root, "go.mod")
	f, err := os.Open(gomod)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	res := &CheckResult{Confidence: 0.95, PackageManager: "go modules", ProjectType: "api_service"}
	version := ""
	fwHits := map[string]string{
		"github.com/gin-gonic/gin": "Gin", "github.com/labstack/echo": "Echo",
		"github.com/gofiber/fiber": "Fiber", "github.com/go-chi/chi": "Chi",
		"github.com/gorilla/mux": "Gorilla Mux", "github.com/spf13/cobra": "Cobra",
		"github.com/urfave/cli": "urfave/cli", "gorm.io/gorm": "GORM",
		"github.com/jmoiron/sqlx": "sqlx", "github.com/jackc/pgx": "pgx",
		"go.mongodb.org/mongo-driver": "Mongo Driver",
	}
	dbMap := map[string]string{
		"gorm.io/gorm": "sql", "github.com/jmoiron/sqlx": "sql",
		"github.com/jackc/pgx": "postgresql", "go.mongodb.org/mongo-driver": "mongodb",
		"github.com/mattn/go-sqlite3": "sqlite", "github.com/lib/pq": "postgresql",
		"github.com/go-sql-driver/mysql": "mysql",
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "go ") && version == "" {
			version = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
		for mod, fw := range fwHits {
			if strings.Contains(line, mod) {
				ver := extractGoVersion(line)
				res.Frameworks = append(res.Frameworks, projctx.Framework{Name: fw, Version: ver, Type: "backend"})
				if db, ok := dbMap[mod]; ok {
					res.DatabaseTypes = append(res.DatabaseTypes, db)
				}
			}
		}
	}
	res.Languages = []projctx.Language{{Name: "Go", Version: version, Percentage: 90}}
	// CLI detection
	for _, fr := range res.Frameworks {
		if fr.Name == "Cobra" || fr.Name == "urfave/cli" {
			res.ProjectType = "cli_tool"
		}
	}
	return res, nil
}

func extractGoVersion(line string) string {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		return strings.Trim(fields[1], "\"")
	}
	return ""
}
