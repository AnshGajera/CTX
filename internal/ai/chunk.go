package ai

import (
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// Chunk is one RAG chunk.
type Chunk struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Source  string `json:"source,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// Chunker splits context into retrievable chunks.
type Chunker struct {
	MaxChars int
	Overlap  int
}

// NewChunker creates a chunker.
func NewChunker() *Chunker {
	return &Chunker{MaxChars: 2000, Overlap: 200}
}

// Chunk splits a ProjectContext into chunks.
func (c *Chunker) Chunk(ctx *projctx.ProjectContext) []Chunk {
	var out []Chunk
	add := func(kind, id, text, source string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		for _, part := range splitWithOverlap(text, c.MaxChars, c.Overlap) {
			out = append(out, Chunk{ID: id, Kind: kind, Text: part, Source: source, Snippet: truncate(part, 280)})
		}
	}
	if ctx.APIs != nil {
		for _, e := range ctx.APIs.Endpoints {
			add("api", e.Method+" "+e.Path,
				e.Method+" "+e.Path+" handler="+e.Handler+" file="+e.File+" middleware="+strings.Join(e.Middleware, ","),
				e.File)
		}
	}
	if ctx.Database != nil {
		for _, m := range ctx.Database.Models {
			var fields []string
			for _, f := range m.Fields {
				fields = append(fields, f.Name+":"+f.Type)
			}
			add("model", "model:"+m.Name, "model "+m.Name+" fields: "+strings.Join(fields, ", ")+" file="+m.File, m.File)
		}
	}
	if ctx.Environment != nil {
		for _, v := range ctx.Environment.Variables {
			add("env", "env:"+v.Name, "env "+v.Name+" category="+v.Category+" required="+boolStr(v.Required)+" "+v.Description, "")
		}
	}
	if ctx.FileStructure != nil {
		for _, k := range ctx.FileStructure.KeyFiles {
			add("file", "file:"+k.Path, "key file "+k.Path+" purpose="+k.Purpose, k.Path)
		}
		for _, p := range ctx.Patterns.Patterns {
			add("pattern", "pattern:"+p.Name, "pattern "+p.Name+": "+p.Description+" examples="+strings.Join(p.Examples, ", "), "")
		}
	}
	if ctx.Architecture != nil {
		add("architecture", "architecture", "pattern="+ctx.Architecture.Pattern+" diagram="+truncate(ctx.Architecture.Diagram, 500), "")
	}
	return out
}

func splitWithOverlap(s string, max, overlap int) []string {
	if len(s) <= max {
		return []string{s}
	}
	var out []string
	for i := 0; i < len(s); i += max - overlap {
		end := i + max
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
		if end == len(s) {
			break
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
