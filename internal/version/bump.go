package version

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Semver represents a semantic version (X.Y.Z[-prerelease][+build]).
type Semver struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// semverRegex matches standard semantic versions (with optional leading 'v').
var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

// ParseSemver parses a version string into a Semver struct.
func ParseSemver(raw string) (*Semver, error) {
	trimmed := strings.TrimSpace(raw)
	matches := semverRegex.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid semantic version: %q", raw)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, err
	}

	return &Semver{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// String formats the Semver back to standard X.Y.Z notation.
func (s *Semver) String() string {
	res := fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
	if s.Prerelease != "" {
		res += "-" + s.Prerelease
	}
	if s.Build != "" {
		res += "+" + s.Build
	}
	return res
}

// Bump increments major, minor, or patch according to semver spec.
func (s *Semver) Bump(part string) error {
	switch strings.ToLower(part) {
	case "major":
		s.Major++
		s.Minor = 0
		s.Patch = 0
		s.Prerelease = ""
		s.Build = ""
	case "minor":
		s.Minor++
		s.Patch = 0
		s.Prerelease = ""
		s.Build = ""
	case "patch":
		s.Patch++
		s.Prerelease = ""
		s.Build = ""
	default:
		return fmt.Errorf("unsupported bump part %q (must be major, minor, or patch)", part)
	}
	return nil
}

// NextVersion computes the next version string from a current version and bump action.
// bumpType can be "major", "minor", "patch", or an explicit new semver version string.
func NextVersion(current string, bumpType string) (string, error) {
	bumpType = strings.TrimSpace(bumpType)
	if bumpType == "" {
		bumpType = "patch"
	}

	// Check if bumpType is an explicit semver string (e.g. "0.2.0" or "v1.0.0")
	if explicit, err := ParseSemver(bumpType); err == nil {
		return explicit.String(), nil
	}

	sem, err := ParseSemver(current)
	if err != nil {
		// If current is "dev" or unknown, fall back to "0.1.0"
		sem = &Semver{Major: 0, Minor: 1, Patch: 0}
	}

	if err := sem.Bump(bumpType); err != nil {
		return "", err
	}
	return sem.String(), nil
}

// DetectCurrentVersion scans the repository to determine the current version.
// It checks in order: internal/version/version.go, _version.py, git tags, or default.
func DetectCurrentVersion(root string) string {
	// 1. Check _version.py
	pyPath := filepath.Join(root, "_version.py")
	if data, err := os.ReadFile(pyPath); err == nil {
		re := regexp.MustCompile(`__version__\s*=\s*["']([^"']+)["']`)
		if m := re.FindStringSubmatch(string(data)); len(m) > 1 {
			if _, perr := ParseSemver(m[1]); perr == nil {
				return m[1]
			}
		}
	}

	// 2. Check internal/version/version.go
	goPath := filepath.Join(root, "internal", "version", "version.go")
	if data, err := os.ReadFile(goPath); err == nil {
		re := regexp.MustCompile(`Version\s*=\s*["']([^"']+)["']`)
		if m := re.FindStringSubmatch(string(data)); len(m) > 1 {
			if _, perr := ParseSemver(m[1]); perr == nil {
				return m[1]
			}
		}
	}

	// 3. Check git tag
	out, err := exec.Command("git", "-C", root, "describe", "--tags", "--abbrev=0").Output()
	if err == nil {
		tag := strings.TrimSpace(string(out))
		if sem, perr := ParseSemver(tag); perr == nil {
			return sem.String()
		}
	}

	if Version != "" && Version != "dev" {
		if sem, perr := ParseSemver(Version); perr == nil {
			return sem.String()
		}
	}

	return "0.1.0"
}

// BumpResult holds the outcome of a bump execution.
type BumpResult struct {
	CurrentVersion string   `json:"current_version"`
	NextVersion    string   `json:"next_version"`
	BumpType       string   `json:"bump_type"`
	FilesUpdated   []string `json:"files_updated"`
	TagCreated     string   `json:"tag_created,omitempty"`
	DryRun         bool     `json:"dry_run"`
}

// Bump executes a version bump across the codebase.
func Bump(root string, bumpType string, dryRun bool, createTag bool, customFiles []string) (*BumpResult, error) {
	current := DetectCurrentVersion(root)
	next, err := NextVersion(current, bumpType)
	if err != nil {
		return nil, fmt.Errorf("calculate next version: %w", err)
	}

	result := &BumpResult{
		CurrentVersion: current,
		NextVersion:    next,
		BumpType:       bumpType,
		DryRun:         dryRun,
	}

	var candidateFiles []string
	// Standard Go version location
	candidateFiles = append(candidateFiles, filepath.Join("internal", "version", "version.go"))
	// Python version file if present
	candidateFiles = append(candidateFiles, "_version.py")
	// Generic version.go if present
	candidateFiles = append(candidateFiles, "version.go")
	// Any user specified files
	candidateFiles = append(candidateFiles, customFiles...)

	seen := make(map[string]bool)

	for _, relPath := range candidateFiles {
		absPath := filepath.Join(root, relPath)
		if seen[absPath] {
			continue
		}
		seen[absPath] = true

		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		content := string(data)
		modified := false

		// Go version pattern: Version = "..."
		goRe := regexp.MustCompile(`(Version\s*=\s*)"[^"]+"`)
		if goRe.MatchString(content) {
			content = goRe.ReplaceAllString(content, fmt.Sprintf(`${1}"%s"`, next))
			modified = true
		}

		// Python version pattern: __version__ = "..."
		pyRe := regexp.MustCompile(`(__version__\s*=\s*)"[^"]+"`)
		if pyRe.MatchString(content) {
			content = pyRe.ReplaceAllString(content, fmt.Sprintf(`${1}"%s"`, next))
			modified = true
		}

		// Package.json version pattern: "version": "..."
		pkgRe := regexp.MustCompile(`("version"\s*:\s*)"[^"]+"`)
		if pkgRe.MatchString(content) {
			content = pkgRe.ReplaceAllString(content, fmt.Sprintf(`${1}"%s"`, next))
			modified = true
		}

		if modified {
			result.FilesUpdated = append(result.FilesUpdated, relPath)
			if !dryRun {
				if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
					return nil, fmt.Errorf("write %s: %w", relPath, err)
				}
			}
		}
	}

	if createTag && !dryRun {
		tagName := "v" + next
		tagCmd := exec.Command("git", "-C", root, "tag", "-a", tagName, "-m", fmt.Sprintf("Release %s", tagName))
		if out, err := tagCmd.CombinedOutput(); err != nil {
			return result, fmt.Errorf("create git tag %s: %w (%s)", tagName, err, strings.TrimSpace(string(out)))
		}
		result.TagCreated = tagName
	} else if createTag && dryRun {
		result.TagCreated = "v" + next + " (dry-run)"
	}

	return result, nil
}
