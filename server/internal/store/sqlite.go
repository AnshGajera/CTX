package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is the database layer for the CTX server.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Migrate creates the schema.
func (s *SQLiteStore) Migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    password    TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS organizations (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS teams (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, slug)
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id     TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('admin', 'member', 'viewer')),
    joined_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    context_id  TEXT UNIQUE NOT NULL,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS project_teams (
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id     TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    access      TEXT NOT NULL DEFAULT 'read' CHECK(access IN ('read', 'write', 'admin')),
    PRIMARY KEY (project_id, team_id)
);

CREATE TABLE IF NOT EXISTS snapshots (
    hash        TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_hash TEXT,
    author      TEXT NOT NULL DEFAULT '',
    message     TEXT NOT NULL DEFAULT '',
    git_commit  TEXT NOT NULL DEFAULT '',
    git_branch  TEXT NOT NULL DEFAULT '',
    context     TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snapshots_project ON snapshots(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS project_head (
    project_id  TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    hash        TEXT NOT NULL REFERENCES snapshots(hash)
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    token_hash  TEXT UNIQUE NOT NULL,
    read_only   INTEGER NOT NULL DEFAULT 1,
    expires_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used   DATETIME
);

CREATE TABLE IF NOT EXISTS share_policies (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_team TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    sections    TEXT NOT NULL DEFAULT '[]',
    level       TEXT NOT NULL DEFAULT 'L1' CHECK(level IN ('L0', 'L1', 'L2', 'L3')),
    created_by  TEXT NOT NULL REFERENCES users(id),
    expires_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, target_team)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    details     TEXT NOT NULL DEFAULT '{}',
    ip_address  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);
`

// --- User operations ---

// User represents a registered user.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Password  string    `json:"-"` // bcrypt hash, never serialized
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser inserts a new user.
func (s *SQLiteStore) CreateUser(email, name, passwordHash string) (*User, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"INSERT INTO users (id, email, name, password, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, email, name, passwordHash, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &User{ID: id, Email: email, Name: name, CreatedAt: now}, nil
}

// GetUserByEmail finds a user by email.
func (s *SQLiteStore) GetUserByEmail(email string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		"SELECT id, email, name, password, created_at FROM users WHERE email = ?", email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID finds a user by ID.
func (s *SQLiteStore) GetUserByID(id string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		"SELECT id, email, name, password, created_at FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// --- Organization operations ---

// Organization represents an org.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateOrg inserts a new organization and a default team.
func (s *SQLiteStore) CreateOrg(name, slug, userID string) (*Organization, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	orgID := uuid.NewString()
	now := time.Now().UTC()
	_, err = tx.Exec(
		"INSERT INTO organizations (id, name, slug, created_by, created_at) VALUES (?, ?, ?, ?, ?)",
		orgID, name, slug, userID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	// Create default team
	teamID := uuid.NewString()
	_, err = tx.Exec(
		"INSERT INTO teams (id, org_id, name, slug, created_at) VALUES (?, ?, ?, ?, ?)",
		teamID, orgID, "Default", "default", now,
	)
	if err != nil {
		return nil, fmt.Errorf("create default team: %w", err)
	}

	// Add creator as admin
	_, err = tx.Exec(
		"INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES (?, ?, 'admin', ?)",
		teamID, userID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("add org admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Organization{ID: orgID, Name: name, Slug: slug, CreatedBy: userID, CreatedAt: now}, nil
}

// ListOrgs returns orgs the user belongs to.
func (s *SQLiteStore) ListOrgs(userID string) ([]Organization, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT o.id, o.name, o.slug, o.created_by, o.created_at
		FROM organizations o
		JOIN teams t ON t.org_id = o.id
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?
		ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orgs []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedBy, &o.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// --- Project operations ---

// Project represents a registered project.
type Project struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ContextID   string    `json:"context_id"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateProject inserts a new project.
func (s *SQLiteStore) CreateProject(orgID, name, description, contextID, userID string) (*Project, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"INSERT INTO projects (id, org_id, name, description, context_id, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, orgID, name, description, contextID, userID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &Project{ID: id, OrgID: orgID, Name: name, Description: description, ContextID: contextID, CreatedBy: userID, CreatedAt: now}, nil
}

// GetProjectByContextID finds a project by its context_id.
func (s *SQLiteStore) GetProjectByContextID(contextID string) (*Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, org_id, name, description, context_id, created_by, created_at FROM projects WHERE context_id = ?",
		contextID,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.ContextID, &p.CreatedBy, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProjectByID finds a project by ID.
func (s *SQLiteStore) GetProjectByID(id string) (*Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, org_id, name, description, context_id, created_by, created_at FROM projects WHERE id = ?",
		id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.ContextID, &p.CreatedBy, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListProjects returns projects the user can access.
func (s *SQLiteStore) ListProjects(userID, orgID string) ([]Project, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT p.id, p.org_id, p.name, p.description, p.context_id, p.created_by, p.created_at
		FROM projects p
		JOIN project_teams pt ON pt.project_id = p.id
		JOIN team_members tm ON tm.team_id = pt.team_id
		WHERE tm.user_id = ? AND p.org_id = ?
		ORDER BY p.name`, userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.ContextID, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// --- Snapshot operations ---

// Snapshot is a context snapshot stored on the server.
type Snapshot struct {
	Hash       string    `json:"hash"`
	ProjectID  string    `json:"project_id"`
	ParentHash string    `json:"parent_hash,omitempty"`
	Author     string    `json:"author"`
	Message    string    `json:"message"`
	GitCommit  string    `json:"git_commit,omitempty"`
	GitBranch  string    `json:"git_branch,omitempty"`
	Context    string    `json:"context"` // JSON string
	CreatedAt  time.Time `json:"created_at"`
}

// PushSnapshot stores a snapshot and updates the project HEAD.
func (s *SQLiteStore) PushSnapshot(snap *Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO snapshots (hash, project_id, parent_hash, author, message, git_commit, git_branch, context, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.Hash, snap.ProjectID, snap.ParentHash, snap.Author, snap.Message,
		snap.GitCommit, snap.GitBranch, snap.Context, snap.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO project_head (project_id, hash) VALUES (?, ?)
		ON CONFLICT(project_id) DO UPDATE SET hash = excluded.hash`,
		snap.ProjectID, snap.Hash,
	)
	if err != nil {
		return fmt.Errorf("update head: %w", err)
	}

	return tx.Commit()
}

// PullSnapshot returns the HEAD snapshot for a project.
func (s *SQLiteStore) PullSnapshot(projectID string) (*Snapshot, error) {
	var snap Snapshot
	err := s.db.QueryRow(`
		SELECT s.hash, s.project_id, s.parent_hash, s.author, s.message, s.git_commit, s.git_branch, s.context, s.created_at
		FROM snapshots s
		JOIN project_head h ON h.hash = s.hash AND h.project_id = s.project_id
		WHERE s.project_id = ?`, projectID,
	).Scan(&snap.Hash, &snap.ProjectID, &snap.ParentHash, &snap.Author, &snap.Message,
		&snap.GitCommit, &snap.GitBranch, &snap.Context, &snap.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// ListSnapshots returns recent snapshots for a project.
func (s *SQLiteStore) ListSnapshots(projectID string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT hash, project_id, parent_hash, author, message, git_commit, git_branch, created_at
		FROM snapshots WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`,
		projectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snaps []Snapshot
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.Hash, &snap.ProjectID, &snap.ParentHash, &snap.Author,
			&snap.Message, &snap.GitCommit, &snap.GitBranch, &snap.CreatedAt); err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}

// --- Access control ---

// UserCanAccessProject checks if a user has access to a project.
func (s *SQLiteStore) UserCanAccessProject(userID, projectID string) (bool, string, error) {
	var access string
	err := s.db.QueryRow(`
		SELECT pt.access FROM project_teams pt
		JOIN team_members tm ON tm.team_id = pt.team_id
		WHERE pt.project_id = ? AND tm.user_id = ?
		ORDER BY CASE pt.access WHEN 'admin' THEN 1 WHEN 'write' THEN 2 ELSE 3 END
		LIMIT 1`, projectID, userID,
	).Scan(&access)
	if err != nil {
		return false, "", err
	}
	return true, access, nil
}

// --- Share Policy operations ---

// SharePolicy defines what context sections can be shared with a target team.
type SharePolicy struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	TargetTeam string    `json:"target_team"`
	Sections   []string  `json:"sections"`
	Level      string    `json:"level"`
	CreatedBy  string    `json:"created_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateSharePolicy creates a sharing policy.
func (s *SQLiteStore) CreateSharePolicy(projectID, targetTeam string, sections []string, level, userID string) (*SharePolicy, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	sectionsJSON, _ := json.Marshal(sections)
	_, err := s.db.Exec(`
		INSERT INTO share_policies (id, project_id, target_team, sections, level, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, target_team) DO UPDATE SET sections=excluded.sections, level=excluded.level`,
		id, projectID, targetTeam, string(sectionsJSON), level, userID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create share policy: %w", err)
	}
	return &SharePolicy{ID: id, ProjectID: projectID, TargetTeam: targetTeam, Sections: sections, Level: level, CreatedBy: userID, CreatedAt: now}, nil
}

// GetSharePolicy returns the share policy for a project-team pair.
func (s *SQLiteStore) GetSharePolicy(projectID, teamID string) (*SharePolicy, error) {
	var sp SharePolicy
	var sectionsJSON string
	err := s.db.QueryRow(`
		SELECT id, project_id, target_team, sections, level, created_by, expires_at, created_at
		FROM share_policies WHERE project_id = ? AND target_team = ?`,
		projectID, teamID,
	).Scan(&sp.ID, &sp.ProjectID, &sp.TargetTeam, &sectionsJSON, &sp.Level, &sp.CreatedBy, &sp.ExpiresAt, &sp.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(sectionsJSON), &sp.Sections)
	return &sp, nil
}

// --- Audit log ---

// LogAudit records an audit event.
func (s *SQLiteStore) LogAudit(userID, action, resource, resourceID, details, ip string) error {
	id := uuid.NewString()
	_, err := s.db.Exec(
		"INSERT INTO audit_log (id, user_id, action, resource, resource_id, details, ip_address, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, userID, action, resource, resourceID, details, ip, time.Now().UTC(),
	)
	return err
}

// --- API Token operations ---

// APIToken represents a stored API token (the actual token is only returned at creation).
type APIToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	TokenHash string     `json:"-"`
	ReadOnly  bool       `json:"read_only"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateAPIToken stores a new API token.
func (s *SQLiteStore) CreateAPIToken(userID, name, tokenHash string, readOnly bool) (*APIToken, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"INSERT INTO api_tokens (id, user_id, name, token_hash, read_only, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, userID, name, tokenHash, readOnly, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return &APIToken{ID: id, UserID: userID, Name: name, ReadOnly: readOnly, CreatedAt: now}, nil
}

// GetAPITokenByHash finds a token by its hash.
func (s *SQLiteStore) GetAPITokenByHash(tokenHash string) (*APIToken, error) {
	var t APIToken
	var readOnly int
	err := s.db.QueryRow(
		"SELECT id, user_id, name, token_hash, read_only, expires_at, created_at FROM api_tokens WHERE token_hash = ?",
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &readOnly, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.ReadOnly = readOnly == 1
	// Update last_used
	_, _ = s.db.Exec("UPDATE api_tokens SET last_used = ? WHERE id = ?", time.Now().UTC(), t.ID)
	return &t, nil
}
