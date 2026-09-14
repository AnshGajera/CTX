package context

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// ProjectManifest tracks project identity on disk.
type ProjectManifest struct {
	Version     int            `json:"version"`
	ProjectName string         `json:"project_name"`
	ProjectRoot string         `json:"project_root"`
	ContextID   string         `json:"context_id"`
	Profile     ProjectProfile `json:"profile"`
	CreatedAt   time.Time      `json:"created_at"`
	LastExtract time.Time      `json:"last_extract"`
}

// NewManifest creates a manifest with a fresh UUID v4.
func NewManifest(projectName, root string, profile ProjectProfile) *ProjectManifest {
	return &ProjectManifest{
		Version:     1,
		ProjectName: projectName,
		ProjectRoot: root,
		ContextID:   uuid.NewString(),
		Profile:     profile,
		CreatedAt:   time.Now().UTC(),
	}
}

// Save writes the manifest to path.
func (m *ProjectManifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// LoadManifest loads a manifest from path.
func LoadManifest(path string) (*ProjectManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m ProjectManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}
