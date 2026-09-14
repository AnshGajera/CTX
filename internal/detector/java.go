package detector

import (
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

type javaChecker struct{}

func (c *javaChecker) Name() string { return "java" }

func (c *javaChecker) Check(root string) (*CheckResult, error) {
	hasMaven := fileExists(root, "pom.xml")
	hasGradle := fileExists(root, "build.gradle") || fileExists(root, "build.gradle.kts")
	if !hasMaven && !hasGradle {
		return nil, nil
	}
	pm := "maven"
	if hasGradle {
		pm = "gradle"
	}
	var data []byte
	if hasMaven {
		data, _ = os.ReadFile(filepath.Join(root, "pom.xml"))
	} else {
		data, _ = os.ReadFile(filepath.Join(root, "build.gradle"))
		if len(data) == 0 {
			data, _ = os.ReadFile(filepath.Join(root, "build.gradle.kts"))
		}
	}
	s := string(data)
	res := &CheckResult{Confidence: 0.85, PackageManager: pm, ProjectType: "api_service"}
	lang := "Java"
	if strings.Contains(s, "kotlin") || fileExists(root, "settings.gradle.kts") {
		lang = "Kotlin"
	}
	for _, fw := range []string{"spring-boot", "quarkus", "micronaut"} {
		if strings.Contains(s, fw) {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: fw, Type: "backend"})
		}
	}
	res.Languages = []projctx.Language{{Name: lang, Percentage: 85}}
	return res, nil
}
