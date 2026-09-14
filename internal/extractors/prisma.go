package extractors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

// PrismaExtractor parses schema.prisma.
type PrismaExtractor struct{ Base }

func NewPrisma(root string) *PrismaExtractor {
	return &PrismaExtractor{Base: NewBase(root)}
}

func (e *PrismaExtractor) Name() string { return "prisma" }

var scalarTypes = map[string]bool{
	"String": true, "Int": true, "Float": true, "Boolean": true,
	"DateTime": true, "Json": true, "Bytes": true, "BigInt": true, "Decimal": true,
}

var (
	reDatasourceProvider = regexp.MustCompile(`provider\s*=\s*"([^"]+)"`)
	reModelStart         = regexp.MustCompile(`^model\s+(\w+)\s*\{`)
	reEnumStart          = regexp.MustCompile(`^enum\s+(\w+)\s*\{`)
	reCompoundIndex      = regexp.MustCompile(`@@index\(\[([^\]]+)\]\)`)
	reCompoundUnique     = regexp.MustCompile(`@@unique\(\[([^\]]+)\]\)`)
	reDefaultAttr        = regexp.MustCompile(`@default\(([^)]+)\)`)
)

func (e *PrismaExtractor) Extract(ctx *projctx.ProjectContext) error {
	schemaPath := e.findSchema()
	if schemaPath == "" {
		return nil
	}
	rel, _ := filepath.Rel(e.Root, schemaPath)
	f, err := os.Open(schemaPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	db := &projctx.DatabaseContext{ORM: "prisma"}
	var current *projctx.DatabaseModel
	inModel := false
	inDatasource := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 128*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "datasource") {
			inDatasource = true
			continue
		}
		if inDatasource {
			if m := reDatasourceProvider.FindStringSubmatch(line); len(m) == 2 {
				db.Type = m[1]
			}
			if strings.Contains(line, "}") {
				inDatasource = false
			}
			continue
		}
		if m := reModelStart.FindStringSubmatch(line); len(m) == 2 {
			inModel = true
			current = &projctx.DatabaseModel{Name: m[1], Table: m[1], File: filepath.ToSlash(rel)}
			continue
		}
		if reEnumStart.MatchString(line) {
			inModel = false
			current = nil
			// skip enum body
			continue
		}
		if inModel && strings.HasPrefix(line, "}") {
			if current != nil {
				db.Models = append(db.Models, *current)
			}
			current = nil
			inModel = false
			continue
		}
		if inModel && current != nil {
			if strings.HasPrefix(line, "@@") {
				if m := reCompoundIndex.FindStringSubmatch(line); len(m) == 2 {
					fields := splitCSV(m[1])
					current.Indexes = append(current.Indexes, projctx.Index{Fields: fields})
				}
				if m := reCompoundUnique.FindStringSubmatch(line); len(m) == 2 {
					fields := splitCSV(m[1])
					current.Indexes = append(current.Indexes, projctx.Index{Fields: fields, Unique: true})
				}
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			field := parsePrismaField(line, current.Name, db)
			if field != nil {
				current.Fields = append(current.Fields, *field)
				if field.Reference != "" {
					db.Relations = append(db.Relations, projctx.Relation{
						From: current.Name, To: field.Reference, Type: "relation", FieldName: field.Name,
					})
				}
				if field.Unique && len(current.Indexes) == 0 {
					// single unique handled on field; no separate index needed
				}
			}
		}
	}
	db.Diagram = mermaidER(db.Models, db.Relations)
	if ctx.Database == nil {
		ctx.Database = db
	} else {
		if ctx.Database.Type == "" {
			ctx.Database.Type = db.Type
		}
		ctx.Database.Models = append(ctx.Database.Models, db.Models...)
		ctx.Database.Relations = append(ctx.Database.Relations, db.Relations...)
		if ctx.Database.Diagram == "" {
			ctx.Database.Diagram = db.Diagram
		}
	}
	return nil
}

func (e *PrismaExtractor) findSchema() string {
	for _, c := range []string{"prisma/schema.prisma", "schema.prisma"} {
		p := filepath.Join(e.Root, c)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	var found string
	_ = filepath.Walk(e.Root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if err == nil && info.IsDir() && ShouldSkipDir(info.Name()) && p != e.Root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(p) == "schema.prisma" {
			found = p
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func parsePrismaField(line, modelName string, db *projctx.DatabaseContext) *projctx.ModelField {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}
	name := parts[0]
	rawType := parts[1]
	attrs := strings.Join(parts[2:], " ")
	nullable := strings.HasSuffix(rawType, "?")
	typ := strings.TrimSuffix(strings.TrimSuffix(rawType, "?"), "[]")
	isArray := strings.HasSuffix(rawType, "[]")
	f := &projctx.ModelField{Name: name, Type: typ, Nullable: nullable}
	if strings.Contains(attrs, "@id") {
		f.PrimaryKey = true
	}
	if strings.Contains(attrs, "@unique") {
		f.Unique = true
	}
	if m := reDefaultAttr.FindStringSubmatch(attrs); len(m) == 2 {
		f.Default = m[1]
	}
	if !scalarTypes[typ] && !isArray {
		f.Reference = typ
	} else if !scalarTypes[typ] && isArray {
		// relation list, still record relation
		f.Reference = typ
		db.Relations = append(db.Relations, projctx.Relation{From: modelName, To: typ, Type: "one-to-many", FieldName: name})
		f.Reference = "" // avoid double-add; relation already added
		// set back so caller doesn't duplicate
		return f
	}
	return f
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mermaidER(models []projctx.DatabaseModel, rels []projctx.Relation) string {
	if len(models) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("erDiagram\n")
	for _, m := range models {
		fmt.Fprintf(&b, "  %s {\n", m.Name)
		for _, f := range m.Fields {
			if f.Reference != "" {
				continue
			}
			fmt.Fprintf(&b, "    %s %s\n", f.Type, f.Name)
		}
		b.WriteString("  }\n")
	}
	for _, r := range rels {
		fmt.Fprintf(&b, "  %s ||--o{ %s : \"%s\"\n", r.From, r.To, r.FieldName)
	}
	return b.String()
}
