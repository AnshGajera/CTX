package detector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectsNestedFullStackApplication(t *testing.T) {
	root := t.TempDir()
	writeDetectorFixture(t, filepath.Join(root, "backend", "requirements.txt"), "fastapi==0.115.0\n")
	writeDetectorFixture(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"next":"14.2.0","react":"18.3.0"}}`)

	profile, err := New(root).Detect()
	if err != nil {
		t.Fatal(err)
	}
	if profile.ProjectType != "web_application" {
		t.Fatalf("expected web_application, got %q", profile.ProjectType)
	}
	var fastAPI, next bool
	for _, framework := range profile.Frameworks {
		fastAPI = fastAPI || strings.EqualFold(framework.Name, "FastAPI")
		next = next || strings.EqualFold(framework.Name, "Next.js")
	}
	if !fastAPI || !next {
		t.Fatalf("missing nested frameworks: %+v", profile.Frameworks)
	}
}

func writeDetectorFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
