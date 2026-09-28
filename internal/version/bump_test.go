package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		major   int
		minor   int
		patch   int
		prerel  string
		build   string
		wantErr bool
	}{
		{"0.1.5", 0, 1, 5, "", "", false},
		{"v1.2.3", 1, 2, 3, "", "", false},
		{"1.0.0-alpha.1", 1, 0, 0, "alpha.1", "", false},
		{"1.0.0+20130313144700", 1, 0, 0, "", "20130313144700", false},
		{"invalid", 0, 0, 0, "", "", true},
		{"1.2", 0, 0, 0, "", "", true},
	}

	for _, tc := range tests {
		s, err := ParseSemver(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseSemver(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if err == nil {
			if s.Major != tc.major || s.Minor != tc.minor || s.Patch != tc.patch || s.Prerelease != tc.prerel || s.Build != tc.build {
				t.Errorf("ParseSemver(%q) = %+v, want major=%d minor=%d patch=%d prerel=%s build=%s",
					tc.input, s, tc.major, tc.minor, tc.patch, tc.prerel, tc.build)
			}
		}
	}
}

func TestNextVersion(t *testing.T) {
	cases := []struct {
		current  string
		bumpType string
		want     string
	}{
		{"0.1.5", "patch", "0.1.6"},
		{"0.1.5", "minor", "0.2.0"},
		{"0.1.5", "major", "1.0.0"},
		{"0.1.5", "1.5.0", "1.5.0"},
		{"0.1.5", "v2.0.0", "2.0.0"},
		{"", "patch", "0.1.1"},
	}

	for _, c := range cases {
		got, err := NextVersion(c.current, c.bumpType)
		if err != nil {
			t.Errorf("NextVersion(%q, %q) error = %v", c.current, c.bumpType, err)
			continue
		}
		if got != c.want {
			t.Errorf("NextVersion(%q, %q) = %q, want %q", c.current, c.bumpType, got, c.want)
		}
	}
}

func TestBumpExecution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock version.go
	verDir := filepath.Join(tmpDir, "internal", "version")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	verGo := filepath.Join(verDir, "version.go")
	if err := os.WriteFile(verGo, []byte(`package version

var (
	Version = "0.1.5"
	Commit  = "none"
)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create mock _version.py
	verPy := filepath.Join(tmpDir, "_version.py")
	if err := os.WriteFile(verPy, []byte(`__version__ = "0.1.5"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dry run test
	resDry, err := Bump(tmpDir, "minor", true, false, nil)
	if err != nil {
		t.Fatalf("Bump dry-run failed: %v", err)
	}
	if resDry.NextVersion != "0.2.0" {
		t.Errorf("expected 0.2.0, got %s", resDry.NextVersion)
	}
	// Verify files not changed
	data, _ := os.ReadFile(verGo)
	if !containsStr(string(data), `"0.1.5"`) {
		t.Fatalf("dry run should not modify file: %s", string(data))
	}

	// Actual run test
	res, err := Bump(tmpDir, "minor", false, false, nil)
	if err != nil {
		t.Fatalf("Bump failed: %v", err)
	}
	if res.NextVersion != "0.2.0" {
		t.Errorf("expected 0.2.0, got %s", res.NextVersion)
	}

	dataGo, _ := os.ReadFile(verGo)
	if !containsStr(string(dataGo), `"0.2.0"`) {
		t.Fatalf("expected version.go to have 0.2.0, got %s", string(dataGo))
	}

	dataPy, _ := os.ReadFile(verPy)
	if !containsStr(string(dataPy), `"0.2.0"`) {
		t.Fatalf("expected _version.py to have 0.2.0, got %s", string(dataPy))
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || filepath.Base(s) == substr || (len(s) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
