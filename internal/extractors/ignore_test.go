package extractors

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestIgnoreBasenameAndDir(t *testing.T) {
	pats := compileIgnorePatterns([]string{"*.log", "secrets/", "*.env"})
	cases := map[string]bool{
		"a.log":                true,
		"sub/dir/b.log":        true,
		"secrets/key.pem":      true,
		"secrets":              true,
		"src/app.ts":           false,
		"src/.env":             true,
		"sub/.env.backup":      false,
		"node_modules/x/index": false,
	}
	for rel, want := range cases {
		if got := matchIgnore(pats, rel); got != want {
			t.Errorf("matchIgnore(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestIgnoreNegationAndAnchors(t *testing.T) {
	pats := compileIgnorePatterns([]string{"*.log", "!keep.log", "/root-only.txt", "build/**/*.map"})
	if !matchIgnore(pats, "a.log") {
		t.Error("a.log should be excluded")
	}
	if matchIgnore(pats, "keep.log") {
		t.Error("keep.log should be re-included by negation")
	}
	if matchIgnore(pats, "sub/keep.log") {
		t.Error("negation should apply at any level")
	}
	if !matchIgnore(pats, "root-only.txt") {
		t.Error("root-anchored file should match at root")
	}
	if !matchIgnore(pats, "build/a/b/c.map") {
		t.Error("** should span directories")
	}
	if matchIgnore(pats, "src/c.map") {
		t.Error("build/** should not match src/")
	}
}

func TestParsePrismaField(t *testing.T) {
	db := &projctx.DatabaseContext{}
	f := parsePrismaField(`id String @id @default(cuid())`, "User", db)
	if f == nil || !f.PrimaryKey || f.Type != "String" {
		t.Fatalf("id field: %+v", f)
	}
	f = parsePrismaField(`email String? @unique`, "User", db)
	if f == nil || !f.Nullable || !f.Unique {
		t.Fatalf("email field: %+v", f)
	}
	f = parsePrismaField(`author User @relation(fields: [authorId], references: [id])`, "Post", db)
	if f == nil || f.Reference != "User" {
		t.Fatalf("relation field: %+v", f)
	}
	// Callers record f.Reference into db.Relations (see prisma.go Extract).
	db.Relations = append(db.Relations, projctx.Relation{From: "Post", To: f.Reference, Type: "relation", FieldName: f.Name})
	if len(db.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(db.Relations))
	}
}
