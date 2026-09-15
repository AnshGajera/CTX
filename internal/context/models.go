package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProjectContext is the root structured context document.
type ProjectContext struct {
	Version       int                     `json:"version"`
	ContextID     string                  `json:"context_id"`
	ProjectName   string                  `json:"project_name"`
	ExtractedAt   time.Time               `json:"extracted_at"`
	ContentHash   string                  `json:"content_hash"`
	Profile       ProjectProfile          `json:"profile"`
	Architecture  *ArchitectureContext  `json:"architecture,omitempty"`
	APIs          *APIContext           `json:"apis,omitempty"`
	Database      *DatabaseContext      `json:"database,omitempty"`
	Dependencies  *DependencyContext    `json:"dependencies,omitempty"`
	Environment   *EnvironmentContext   `json:"environment,omitempty"`
	FileStructure *FileStructureContext `json:"file_structure,omitempty"`
	BusinessRules *BusinessRuleContext  `json:"business_rules,omitempty"`
	Decisions     *DecisionContext      `json:"decisions,omitempty"`
	Patterns      *PatternContext       `json:"patterns,omitempty"`
	CurrentState  *ProjectStateContext  `json:"current_state,omitempty"`

	// Cross-project awareness
	ExternalDeps     []ExternalServiceDep `json:"external_deps,omitempty"`
	SharedInterfaces []SharedInterface    `json:"shared_interfaces,omitempty"`

	// AI agent memory — conventions discovered by AI agents during development
	AIConventions []AIConvention `json:"ai_conventions,omitempty"`

	// Section-level freshness metadata
	SectionMeta map[string]*SectionMeta `json:"section_meta,omitempty"`

	// Health score (computed, not persisted in context.json)
	HealthScore *HealthScore `json:"health_score,omitempty"`
}

// Language is a detected programming language share.
type Language struct {
	Name       string  `json:"name"`
	Version    string  `json:"version,omitempty"`
	Percentage float64 `json:"percentage"`
}

// Framework is a detected framework/library.
type Framework struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
}

// ProjectProfile describes detected project identity.
type ProjectProfile struct {
	Languages      []Language  `json:"languages"`
	Frameworks     []Framework `json:"frameworks"`
	PackageManager string      `json:"package_manager,omitempty"`
	ProjectType    string      `json:"project_type,omitempty"`
	IsMonorepo     bool        `json:"is_monorepo"`
	EntryPoints    []string    `json:"entry_points,omitempty"`
	DatabaseTypes  []string    `json:"database_types,omitempty"`
	ContainerType  string      `json:"container_type,omitempty"`
}

// ArchLayer is one architecture layer.
type ArchLayer struct {
	Name        string   `json:"name"`
	Directories []string `json:"directories"`
	Description string   `json:"description,omitempty"`
}

// ServiceDefinition is a deployable service.
type ServiceDefinition struct {
	Name         string   `json:"name"`
	Type         string   `json:"type,omitempty"`
	Directory    string   `json:"directory,omitempty"`
	Port         int      `json:"port,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// CommPattern describes service communication.
type CommPattern struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Protocol string `json:"protocol,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

// ArchitectureContext holds architecture info.
type ArchitectureContext struct {
	Pattern       string              `json:"pattern,omitempty"`
	Layers        []ArchLayer         `json:"layers,omitempty"`
	Services      []ServiceDefinition `json:"services,omitempty"`
	Communication []CommPattern       `json:"communication,omitempty"`
	Diagram       string              `json:"diagram,omitempty"`
}

// APIParam is a single endpoint parameter.
type APIParam struct {
	Name     string `json:"name"`
	In       string `json:"in,omitempty"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required"`
}

// SchemaDefinition is a JSON-schema-like type.
type SchemaDefinition struct {
	Type       string                      `json:"type,omitempty"`
	Properties map[string]SchemaDefinition `json:"properties,omitempty"`
	Items      *SchemaDefinition           `json:"items,omitempty"`
	Required   []string                    `json:"required,omitempty"`
}

// APIEndpoint is one HTTP endpoint.
type APIEndpoint struct {
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Handler      string            `json:"handler,omitempty"`
	File         string            `json:"file,omitempty"`
	Line         int               `json:"line,omitempty"`
	Description  string            `json:"description,omitempty"`
	Parameters   []APIParam        `json:"parameters,omitempty"`
	RequestBody  *SchemaDefinition `json:"request_body,omitempty"`
	Response     *SchemaDefinition `json:"response,omitempty"`
	Middleware   []string          `json:"middleware,omitempty"`
	AuthRequired bool              `json:"auth_required"`
}

// APIContext holds all endpoints.
type APIContext struct {
	Endpoints []APIEndpoint `json:"endpoints"`
	AuthType  string        `json:"auth_type,omitempty"`
	BaseURL   string        `json:"base_url,omitempty"`
	Version   string        `json:"version,omitempty"`
}

// ModelField is one DB column/field.
type ModelField struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
	Unique     bool   `json:"unique"`
	Default    string `json:"default,omitempty"`
	Reference  string `json:"reference,omitempty"`
}

// Index is a DB index.
type Index struct {
	Fields []string `json:"fields"`
	Unique bool     `json:"unique"`
}

// DatabaseModel is one table/model.
type DatabaseModel struct {
	Name    string       `json:"name"`
	Table   string       `json:"table,omitempty"`
	File    string       `json:"file,omitempty"`
	Fields  []ModelField `json:"fields,omitempty"`
	Indexes []Index      `json:"indexes,omitempty"`
}

// Migration is one migration file.
type Migration struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Relation is a model relation.
type Relation struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type,omitempty"`
	FieldName string `json:"field_name,omitempty"`
}

// DatabaseContext holds schema info.
type DatabaseContext struct {
	Type       string          `json:"type,omitempty"`
	ORM        string          `json:"orm,omitempty"`
	Models     []DatabaseModel `json:"models,omitempty"`
	Migrations []Migration     `json:"migrations,omitempty"`
	Relations  []Relation      `json:"relations,omitempty"`
	Diagram    string          `json:"diagram,omitempty"`
}

// Dependency is one external dependency.
type Dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	Category string `json:"category,omitempty"`
	Critical bool   `json:"critical"`
}

// InternalDep is an intra-repo dependency.
type InternalDep struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type,omitempty"`
}

// DependencyContext holds deps.
type DependencyContext struct {
	Direct   []Dependency  `json:"direct,omitempty"`
	Dev      []Dependency  `json:"dev,omitempty"`
	Internal []InternalDep `json:"internal,omitempty"`
}

// EnvVariable is one env var (never a secret value).
type EnvVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Example     string `json:"example,omitempty"`
	Category    string `json:"category,omitempty"`
	Required    bool   `json:"required"`
}

// EnvironmentContext holds env structure.
type EnvironmentContext struct {
	Variables []EnvVariable `json:"variables,omitempty"`
	Required  []string      `json:"required,omitempty"`
	Profiles  []string      `json:"profiles,omitempty"`
}

// DirectoryNode is a tree node.
type DirectoryNode struct {
	Name      string           `json:"name"`
	Purpose   string           `json:"purpose,omitempty"`
	Children  []*DirectoryNode `json:"children,omitempty"`
	FileCount int              `json:"file_count"`
}

// KeyFile is an important file.
type KeyFile struct {
	Path       string `json:"path"`
	Purpose    string `json:"purpose,omitempty"`
	Importance string `json:"importance,omitempty"`
}

// NamingConvention is a file naming pattern.
type NamingConvention struct {
	Pattern     string `json:"pattern"`
	Example     string `json:"example,omitempty"`
	Description string `json:"description,omitempty"`
}

// FileStructureContext holds tree info.
type FileStructureContext struct {
	Tree        *DirectoryNode     `json:"tree,omitempty"`
	KeyFiles    []KeyFile          `json:"key_files,omitempty"`
	Conventions []NamingConvention `json:"conventions,omitempty"`
}

// BusinessRule is one domain rule.
type BusinessRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Files       []string `json:"files,omitempty"`
	Category    string   `json:"category,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
}

// BusinessRuleContext holds rules.
type BusinessRuleContext struct {
	Rules []BusinessRule `json:"rules,omitempty"`
}

// TechnicalDecision is one ADR-like decision.
type TechnicalDecision struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Status       string   `json:"status,omitempty"`
	Date         string   `json:"date,omitempty"`
	Reasoning    string   `json:"reasoning,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// DecisionContext holds decisions.
type DecisionContext struct {
	Decisions []TechnicalDecision `json:"decisions,omitempty"`
}

// CodePattern is one detected pattern.
type CodePattern struct {
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	Frequency   int      `json:"frequency"`
}

// PatternContext holds patterns.
type PatternContext struct {
	Patterns []CodePattern `json:"patterns,omitempty"`
}

// TODO is one todo comment.
type TODO struct {
	Text string `json:"text"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// ProjectStateContext holds git/working state.
type ProjectStateContext struct {
	GitBranch       string   `json:"git_branch,omitempty"`
	LastCommit      string   `json:"last_commit,omitempty"`
	LastCommitMsg   string   `json:"last_commit_msg,omitempty"`
	DirtyFiles      int      `json:"dirty_files"`
	RecentlyChanged []string `json:"recently_changed,omitempty"`
	ActiveAreas     []string `json:"active_areas,omitempty"`
	TODOs           []TODO   `json:"todos,omitempty"`
}

// --- Cross-project & AI agent types ---

// ExternalServiceDep represents a dependency on another CTX-tracked project.
type ExternalServiceDep struct {
	ProjectID   string   `json:"project_id,omitempty"`
	ProjectName string   `json:"project_name"`
	UsedAPIs    []string `json:"used_apis,omitempty"`
	UsedModels  []string `json:"used_models,omitempty"`
	Protocol    string   `json:"protocol,omitempty"`
}

// SharedInterface declares what this project exposes to other services.
type SharedInterface struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Endpoints   []string `json:"endpoints,omitempty"`
	Events      []string `json:"events,omitempty"`
	Description string   `json:"description,omitempty"`
}

// AIConvention is a pattern/convention discovered by an AI agent.
type AIConvention struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	Confidence  float64   `json:"confidence"`
	DiscoveredAt time.Time `json:"discovered_at"`
	Examples    []string  `json:"examples,omitempty"`
}

// SectionMeta holds freshness and quality metadata for a context section.
// ExtractedAt and Staleness are volatile and excluded from CanonicalHash;
// Source/Extractors/Confidence/ItemCount are content and hashed.
type SectionMeta struct {
	ExtractedAt time.Time `json:"extracted_at"`
	Staleness   string    `json:"staleness"`
	Confidence  float64   `json:"confidence"`
	Source      string    `json:"source"`
	Extractors  []string  `json:"extractors,omitempty"`
	ItemCount   int       `json:"item_count"`
}

// HealthScore represents overall context quality.
type HealthScore struct {
	Score        int            `json:"score"`
	Grade        string         `json:"grade"`
	Checks       []HealthCheck  `json:"checks"`
	Tips         []string       `json:"tips,omitempty"`
}

// HealthCheck is one item in the health report.
type HealthCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Score    int    `json:"score"`
	MaxScore int    `json:"max_score"`
}

// Save writes the context to ctxDir/context.json.
func (c *ProjectContext) Save(ctxDir string) error {
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return fmt.Errorf("create ctx dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}
	path := filepath.Join(ctxDir, "context.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write context: %w", err)
	}
	return nil
}

// LoadContext loads .ctx/context.json from root.
func LoadContext(root string) (*ProjectContext, error) {
	path := filepath.Join(root, ".ctx", "context.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read context: %w", err)
	}
	var ctx ProjectContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parse context: %w", err)
	}
	return &ctx, nil
}
