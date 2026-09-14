package detector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

type dockerChecker struct{}

func (c *dockerChecker) Name() string { return "docker" }

func (c *dockerChecker) Check(root string) (*CheckResult, error) {
	hasDockerfile := fileExists(root, "Dockerfile")
	composePath := ""
	for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if fileExists(root, n) {
			composePath = filepath.Join(root, n)
			break
		}
	}
	if !hasDockerfile && composePath == "" {
		return nil, nil
	}
	res := &CheckResult{Confidence: 0.7, ContainerType: "docker"}
	if composePath != "" {
		services, ports := parseCompose(composePath)
		for _, s := range services {
			res.Frameworks = append(res.Frameworks, projctx.Framework{Name: "docker:" + s, Type: "infrastructure"})
		}
		_ = ports
	}
	return res, nil
}

func parseCompose(path string) ([]string, []int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	var services []string
	var ports []int
	inServices := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "services:" {
			inServices = true
			continue
		}
		if inServices {
			// top-level service names are indented by exactly 2 spaces
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
				name := strings.TrimSuffix(trimmed, ":")
				if name != "" && !strings.Contains(name, " ") {
					services = append(services, name)
				}
			}
			if strings.Contains(trimmed, "ports:") {
				continue
			}
			// crude port extraction: "8080:80"
			if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, ":") {
				parts := strings.Split(strings.TrimPrefix(trimmed, "- "), ":")
				if p, err := strconv.Atoi(strings.Trim(strings.Trim(parts[0], "\"' "), " ")); err == nil {
					ports = append(ports, p)
				}
			}
		}
		if len(services) > 20 {
			break
		}
	}
	return services, ports
}
