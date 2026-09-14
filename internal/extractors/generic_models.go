package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// GenericModelsExtractor covers non-Prisma ORMs: Mongoose, TypeORM,
// SQLAlchemy, Django, and GORM. It complements the Prisma extractor.
type GenericModelsExtractor struct{ Base }

func NewGenericModels(root string) *GenericModelsExtractor {
	return &GenericModelsExtractor{Base: NewBase(root)}
}

func (e *GenericModelsExtractor) Name() string { return "generic-models" }

var (
	reMongooseSchema = regexp.MustCompile(`new\s+(?:mongoose\.)?Schema\s*\(\s*\{?`)
	reMongooseModel  = regexp.MustCompile(`(?:mongoose\.)?model\s*\(\s*['"](\w+)['"]`)
	reMongooseField  = regexp.MustCompile(`^\s*(\w+)\s*:\s*(?:\{[^}]*type\s*:\s*(\w+)|(\w+))`)
	reMongooseRef    = regexp.MustCompile(`ref\s*:\s*['"](\w+)['"]`)
	reTypeORMEntity  = regexp.MustCompile(`@Entity\s*(\([^)]*\))?`)
	reTSClass        = regexp.MustCompile(`(?:export\s+)?(?:default\s+)?class\s+(\w+)`)
	reTSField        = regexp.MustCompile(`^\s*(\w+)[?!]?\s*:\s*([\w\[\]<>]+)`)
	rePyClass        = regexp.MustCompile(`^class\s+(\w+)\s*\(([^)]*)\)`)
	reSqlaColumn     = regexp.MustCompile(`^\s*(\w+)\s*=\s*(?:db\.)?Column\s*\(\s*(\w+)`)
	reDjangoField    = regexp.MustCompile(`^\s*(\w+)\s*=\s*models\.(\w+)\s*\(`)
	reGoStruct       = regexp.MustCompile(`^type\s+(\w+)\s+struct\s*\{`)
	reGoField        = regexp.MustCompile(`^\s*(\w+)\s+([\w\[\]\*\.]+)\s*` + "`" + `([^` + "`" + `]*)` + "`")
	reGormTag        = regexp.MustCompile(`gorm:"([^"]*)"`)
)

func isMigrationPath(rel string) bool {
	lower := filepathToSlash(strings.ToLower(rel))
	if !(strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".py") ||
		strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") ||
		strings.HasSuffix(lower, ".go")) {
		return false
	}
	return strings.Contains(lower, "/migrations/") || strings.Contains(lower, "/migrate/") ||
		strings.Contains(lower, "/db/migrate/") || strings.Contains(lower, "alembic/versions/")
}

func (e *GenericModelsExtractor) Extract(ctx *projctx.ProjectContext) error {
	db := &projctx.DatabaseContext{}
	if ctx.Database != nil {
		db = ctx.Database
	} else {
		ctx.Database = db
	}
	seen := map[string]bool{}
	for _, m := range db.Models {
		seen[m.Name] = true
	}
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if isMigrationPath(rel) {
			db.Migrations = append(db.Migrations, projctx.Migration{
				Name: strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)),
				File: filepathToSlash(rel),
			})
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if info.Size() > 1_000_000 {
			return nil
		}
		switch ext {
		case ".js", ".ts", ".tsx", ".jsx", ".mjs":
			e.parseJS(path, rel, db, seen)
		case ".py":
			e.parsePython(path, rel, db, seen)
		case ".go":
			e.parseGo(path, rel, db, seen)
		}
		return nil
	})
	if db.ORM == "" && len(db.Models) > 0 {
		db.ORM = inferORM(db.Models)
	}
	if db.Diagram == "" && len(db.Models) > 0 {
		db.Diagram = genericMermaid(db.Models, db.Relations)
	}
	return nil
}

func inferORM(models []projctx.DatabaseModel) string {
	// ORM is per-model in practice; report the most common source.
	counts := map[string]int{}
	for _, m := range models {
		if strings.HasPrefix(m.Table, "mongoose:") {
			counts["mongoose"]++
		}
	}
	best, bestN := "", 0
	for k, n := range counts {
		if n > bestN {
			best, bestN = k, n
		}
	}
	return best
}

func (e *GenericModelsExtractor) parseJS(path, rel string, db *projctx.DatabaseContext, seen map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	type line struct {
		no   int
		text string
	}
	var lines []line
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 128*1024), 2*1024*1024)
	no := 0
	for sc.Scan() {
		no++
		lines = append(lines, line{no, sc.Text()})
	}
	full := ""
	for _, l := range lines {
		full += l.text + "\n"
	}
	isMongoose := strings.Contains(full, "mongoose") || reMongooseSchema.MatchString(full)
	isTypeORM := strings.Contains(full, "@Entity")

	// Mongoose: model("Name", schema)
	for _, m := range reMongooseModel.FindAllStringSubmatch(full, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		model := projctx.DatabaseModel{Name: name, Table: "mongoose:" + name, File: filepathToSlash(rel)}
		// fields: scan schema literal with paren/brace depth tracking
		depth := 0
		inSchema := false
		for _, l := range lines {
			if !inSchema {
				if reMongooseSchema.MatchString(l.text) {
					inSchema = true
					depth = strings.Count(l.text, "{") + strings.Count(l.text, "(") -
						strings.Count(l.text, "}") - strings.Count(l.text, ")")
				}
				continue
			}
			depth += strings.Count(l.text, "{") + strings.Count(l.text, "(") -
				strings.Count(l.text, "}") - strings.Count(l.text, ")")
			if depth <= 0 {
				break
			}
			if fm := reMongooseField.FindStringSubmatch(l.text); len(fm) > 0 {
				typ := fm[2]
				if typ == "" {
					typ = fm[3]
				}
				field := projctx.ModelField{Name: fm[1], Type: typ}
				model.Fields = append(model.Fields, field)
				if rm := reMongooseRef.FindStringSubmatch(l.text); len(rm) == 2 {
					db.Relations = append(db.Relations, projctx.Relation{
						From: name, To: rm[1], Type: "relation", FieldName: fm[1],
					})
					model.Fields[len(model.Fields)-1].Reference = rm[1]
				}
			}
		}
		db.Models = append(db.Models, model)
	}
	if !isMongoose && !isTypeORM {
		return
	}
	// TypeORM: @Entity() class X { @Column() name: type }
	for i, l := range lines {
		if !reTypeORMEntity.MatchString(l.text) {
			continue
		}
		for j := i; j < len(lines) && j < i+6; j++ {
			if cm := reTSClass.FindStringSubmatch(lines[j].text); len(cm) == 2 {
				name := cm[1]
				if seen[name] {
					break
				}
				seen[name] = true
				model := projctx.DatabaseModel{Name: name, Table: name, File: filepathToSlash(rel)}
				depth := 0
				started := false
				for k := j; k < len(lines); k++ {
					t := lines[k].text
					depth += strings.Count(t, "{") - strings.Count(t, "}")
					if strings.Contains(t, "{") {
						started = true
					}
					if started {
						if fm := reTSField.FindStringSubmatch(t); len(fm) == 3 && !strings.Contains(t, "(") {
							model.Fields = append(model.Fields, projctx.ModelField{Name: fm[1], Type: fm[2], Nullable: strings.Contains(fm[1], "?")})
						}
					}
					if started && depth <= 0 {
						break
					}
				}
				db.Models = append(db.Models, model)
				break
			}
		}
	}
	_ = isMongoose
}

func (e *GenericModelsExtractor) parsePython(path, rel string, db *projctx.DatabaseContext, seen map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 128*1024), 2*1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	for i, l := range lines {
		m := rePyClass.FindStringSubmatch(l)
		if len(m) != 3 {
			continue
		}
		name, bases := m[1], m[2]
		kind := ""
		switch {
		case strings.Contains(bases, "db.Model") || strings.Contains(bases, "Base") && strings.Contains(strings.Join(lines, "\n"), "Column"):
			kind = "sqlalchemy"
		case strings.Contains(bases, "models.Model") || strings.Contains(bases, "Model"):
			// disambiguate django vs sqlalchemy by field syntax below
		}
		if kind == "" {
			// peek at body for field syntax
			for k := i + 1; k < len(lines) && k < i+30; k++ {
				if reDjangoField.MatchString(lines[k]) {
					kind = "django"
					break
				}
				if reSqlaColumn.MatchString(lines[k]) {
					kind = "sqlalchemy"
					break
				}
			}
		}
		if kind == "" || seen[name] {
			continue
		}
		seen[name] = true
		model := projctx.DatabaseModel{Name: name, Table: name, File: filepathToSlash(rel)}
		for k := i + 1; k < len(lines); k++ {
			t := lines[k]
			if t != "" && !strings.HasPrefix(t, " ") && !strings.HasPrefix(t, "\t") {
				break
			}
			if kind == "django" {
				if fm := reDjangoField.FindStringSubmatch(t); len(fm) == 3 {
					model.Fields = append(model.Fields, projctx.ModelField{Name: fm[1], Type: "django:" + fm[2]})
				}
			} else {
				if fm := reSqlaColumn.FindStringSubmatch(t); len(fm) == 3 {
					model.Fields = append(model.Fields, projctx.ModelField{Name: fm[1], Type: fm[2]})
				}
			}
		}
		db.Models = append(db.Models, model)
	}
}

func (e *GenericModelsExtractor) parseGo(path, rel string, db *projctx.DatabaseContext, seen map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 128*1024), 2*1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	for i, l := range lines {
		m := reGoStruct.FindStringSubmatch(l)
		if len(m) != 2 {
			continue
		}
		name := m[1]
		// collect body, require at least one gorm tag
		var fields []projctx.ModelField
		hasGorm := false
		pks := map[string]bool{}
		for k := i + 1; k < len(lines); k++ {
			t := lines[k]
			if strings.HasPrefix(strings.TrimSpace(t), "}") {
				break
			}
			fm := reGoField.FindStringSubmatch(t)
			if len(fm) == 0 {
				continue
			}
			tag := ""
			if len(fm) >= 4 {
				tag = fm[3]
			}
			if strings.Contains(tag, "gorm:") {
				hasGorm = true
			}
			field := projctx.ModelField{Name: fm[1], Type: fm[2]}
			if gm := reGormTag.FindStringSubmatch(tag); len(gm) == 2 {
				for _, part := range strings.Split(gm[1], ";") {
					part = strings.TrimSpace(part)
					if part == "primaryKey" || part == "primary_key" {
						field.PrimaryKey = true
						pks[fm[1]] = true
					}
					if part == "unique" || strings.HasPrefix(part, "uniqueIndex") {
						field.Unique = true
					}
				}
			}
			fields = append(fields, field)
		}
		_ = pks
		if !hasGorm || seen[name] {
			continue
		}
		seen[name] = true
		db.Models = append(db.Models, projctx.DatabaseModel{Name: name, Table: name, File: filepathToSlash(rel), Fields: fields})
	}
}

func genericMermaid(models []projctx.DatabaseModel, rels []projctx.Relation) string {
	if len(models) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("erDiagram\n")
	for _, m := range models {
		b.WriteString("  " + m.Name + " {\n")
		for _, f := range m.Fields {
			if f.Reference != "" {
				continue
			}
			typ := strings.ReplaceAll(strings.ReplaceAll(f.Type, " ", "_"), ":", "_")
			b.WriteString("    " + typ + " " + f.Name + "\n")
		}
		b.WriteString("  }\n")
	}
	for _, r := range rels {
		b.WriteString("  " + r.From + " ||--o{ " + r.To + " : \"" + r.FieldName + "\"\n")
	}
	return b.String()
}
