package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// ArchitectureExtractor infers layers, services, and a Mermaid diagram
// from directory layout and docker-compose — evidence-based only, no
// fabricated edges (depends_on is the only communication source).
type ArchitectureExtractor struct{ Base }

func NewArchitecture(root string) *ArchitectureExtractor {
	return &ArchitectureExtractor{Base: NewBase(root)}
}

func (e *ArchitectureExtractor) Name() string { return "architecture" }

func (e *ArchitectureExtractor) Extract(ctx *projctx.ProjectContext) error {
	layers := e.inferLayers()
	services, edges := e.parseComposeServices()
	pattern := inferPattern(layers, services)
	arch := &projctx.ArchitectureContext{
		Pattern:       pattern,
		Layers:        layers,
		Services:      services,
		Communication: edges,
	}
	arch.Diagram = mermaidFlow(layers, services, edges)
	if ctx.Architecture == nil {
		ctx.Architecture = arch
	}
	return nil
}

func (e *ArchitectureExtractor) inferLayers() []projctx.ArchLayer {
	entries, err := os.ReadDir(e.Root)
	if err != nil {
		return nil
	}
	var layers []projctx.ArchLayer
	for _, en := range entries {
		if !en.IsDir() {
			continue
		}
		name := en.Name()
		if ShouldSkipDir(name) {
			continue
		}
		if b := e; b.ShouldExclude(name) {
			continue
		}
		purpose := inferDirectoryPurpose(name)
		if purpose == "" {
			purpose = "project module"
		}
		layers = append(layers, projctx.ArchLayer{
			Name:        name,
			Directories: []string{name},
			Description: purpose,
		})
		if len(layers) >= 16 {
			break
		}
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i].Name < layers[j].Name })
	return layers
}

func inferPattern(layers []projctx.ArchLayer, services []projctx.ServiceDefinition) string {
	names := map[string]bool{}
	for _, l := range layers {
		names[strings.ToLower(l.Name)] = true
	}
	switch {
	case names["cmd"] && names["pkg"] && names["internal"]:
		return "go-standard-layout"
	case names["app"] && names["components"]:
		return "nextjs-app-router"
	case names["controllers"] && names["models"] && (names["routes"] || names["views"]):
		return "mvc"
	case names["services"] && names["repositories"]:
		return "layered-service-repository"
	case names["handlers"] && (names["domain"] || names["usecases"]):
		return "clean-architecture"
	case names["features"] || names["modules"]:
		return "modular-monolith"
	case len(services) > 1:
		return "multi-service"
	case names["src"] && names["tests"]:
		return "src-tests-split"
	default:
		return "single-package"
	}
}

type composeSvc struct {
	name string
	port int
	deps []string
}

func (e *ArchitectureExtractor) parseComposeServices() ([]projctx.ServiceDefinition, []projctx.CommPattern) {
	var composePath string
	for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if p := filepath.Join(e.Root, n); fileExistsExt(p) {
			composePath = p
			break
		}
	}
	if composePath == "" {
		return nil, nil
	}
	f, err := os.Open(composePath)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	var svcs []composeSvc
	var cur *composeSvc
	inServices := false
	inDepends := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "services:" {
			inServices = true
			continue
		}
		if !inServices {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSuffix(trimmed, ":")
			if name != "" && !strings.Contains(name, " ") {
				svcs = append(svcs, composeSvc{name: name})
				cur = &svcs[len(svcs)-1]
				inDepends = false
			}
			continue
		}
		if cur == nil {
			continue
		}
		if indent == 4 && strings.HasSuffix(trimmed, ":") {
			inDepends = strings.HasPrefix(trimmed, "depends_on")
			if strings.HasPrefix(trimmed, "ports") {
				inDepends = false
			}
			continue
		}
		if inDepends && strings.HasPrefix(trimmed, "- ") {
			dep := strings.Trim(strings.TrimPrefix(trimmed, "- "), "\"' ")
			if dep != "" {
				cur.deps = append(cur.deps, dep)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, ":") {
			parts := strings.SplitN(strings.TrimPrefix(trimmed, "- "), ":", 2)
			if p, err := strconv.Atoi(strings.Trim(strings.Trim(parts[0], "\"' "), " ")); err == nil && cur.port == 0 {
				cur.port = p
			}
		}
	}
	var out []projctx.ServiceDefinition
	var edges []projctx.CommPattern
	for _, s := range svcs {
		out = append(out, projctx.ServiceDefinition{
			Name: s.name, Type: "container", Port: s.port,
			Dependencies: s.deps, Description: "docker-compose service",
		})
		for _, d := range s.deps {
			edges = append(edges, projctx.CommPattern{From: s.name, To: d, Protocol: "http", Pattern: "sync"})
		}
	}
	return out, edges
}

func fileExistsExt(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func mermaidFlow(layers []projctx.ArchLayer, services []projctx.ServiceDefinition, edges []projctx.CommPattern) string {
	if len(layers) == 0 && len(services) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	safe := func(s string) string {
		return strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, s)
	}
	for _, l := range layers {
		id := "L_" + safe(l.Name)
		b.WriteString("  " + id + "[" + l.Name + "]\n")
	}
	for _, s := range services {
		id := "S_" + safe(s.Name)
		b.WriteString("  " + id + "((" + s.Name + "))\n")
	}
	for _, e := range edges {
		b.WriteString("  S_" + safe(e.From) + " --> S_" + safe(e.To) + "\n")
	}
	return b.String()
}
