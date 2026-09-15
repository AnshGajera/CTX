package engine

import (
	"testing"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/extractors"
)

type stubExtractor struct{ name string }

func (s stubExtractor) Name() string { return s.name }
func (s stubExtractor) Extract(_ *projctx.ProjectContext) error { return nil }

var _ extractors.Extractor = stubExtractor{}

func TestStampSectionMeta(t *testing.T) {
	at := time.Now().UTC()
	ctx := &projctx.ProjectContext{
		ExtractedAt: at,
		APIs:        &projctx.APIContext{},
		Database:    &projctx.DatabaseContext{},
	}
	ex := []extractors.Extractor{
		stubExtractor{"go-api"},
		stubExtractor{"sidecar-ast"},
		stubExtractor{"prisma"},
	}
	stampSectionMeta(ctx, ex, []string{"", "", "boom"})
	if ctx.SectionMeta == nil {
		t.Fatal("expected section meta")
	}
	apis := ctx.SectionMeta["apis"]
	if apis == nil {
		t.Fatal("expected apis meta")
	}
	if apis.Source != "regex" {
		t.Fatalf("sidecar failed so source must be regex, got %q", apis.Source)
	}
	if apis.Confidence != 0.5 {
		t.Fatalf("1 of 2 ok => 0.5, got %v", apis.Confidence)
	}
	if !apis.ExtractedAt.Equal(at) {
		t.Fatal("meta timestamp must match extraction time")
	}
	db := ctx.SectionMeta["database"]
	if db == nil || db.Confidence != 0 {
		t.Fatalf("failed prisma must stamp 0 confidence, got %+v", db)
	}
	if _, ok := ctx.SectionMeta["architecture"]; ok {
		t.Fatal("empty sections must not be stamped")
	}
}
