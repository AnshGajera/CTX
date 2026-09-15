package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// EnvironmentExtractor extracts env var structure (never values from .env).
type EnvironmentExtractor struct{ Base }

func NewEnvironment(root string) *EnvironmentExtractor {
	return &EnvironmentExtractor{Base: NewBase(root)}
}

func (e *EnvironmentExtractor) Name() string { return "environment" }

var (
	reProcessEnvBracket = regexp.MustCompile(`process\.env\[['"]([A-Z0-9_]+)['"]\]`)
	reProcessEnvDot     = regexp.MustCompile(`process\.env\.([A-Z0-9_]+)`)
	reOsEnviron         = regexp.MustCompile(`os\.environ\[['"]([A-Z0-9_]+)['"]\]`)
	reOsGetenv          = regexp.MustCompile(`os\.getenv\(['"]([A-Z0-9_]+)['"]\)`)
	reGoGetenv          = regexp.MustCompile(`os\.Getenv\(["']([A-Z0-9_]+)["']\)`)
	reRustEnv           = regexp.MustCompile(`env::var\(["']([A-Z0-9_]+)["']\)`)
	reRubyEnv           = regexp.MustCompile(`ENV\[['"]([A-Z0-9_]+)['"]\]`)
	reJavaGetenv        = regexp.MustCompile(`System\.getenv\(["']([A-Z0-9_]+)["']\)`)
	reViteEnv           = regexp.MustCompile(`import\.meta\.env\.([A-Z0-9_]+)`)
	reDockerEnv         = regexp.MustCompile(`^\s*-\s*([A-Z0-9_]+)=`)
	reDockerfileEnv     = regexp.MustCompile(`^ENV\s+([A-Z0-9_]+)`)
	reNextPublic        = regexp.MustCompile(`NEXT_PUBLIC_[A-Z0-9_]+`)
)

func categorizeEnv(name string) string {
	n := strings.ToUpper(name)
	switch {
	case strings.Contains(n, "DATABASE") || strings.Contains(n, "DB_") || strings.Contains(n, "POSTGRES") || strings.Contains(n, "MYSQL") || strings.Contains(n, "MONGO") || strings.Contains(n, "REDIS") || strings.HasPrefix(n, "DB"):
		return "database"
	case strings.Contains(n, "AUTH") || strings.Contains(n, "JWT") || strings.Contains(n, "SESSION") || strings.Contains(n, "OAUTH") || strings.Contains(n, "SECRET") || strings.Contains(n, "TOKEN") || strings.Contains(n, "CLERK") || strings.Contains(n, "AUTH0") || strings.Contains(n, "NEXTAUTH") || strings.Contains(n, "API_KEY") || strings.Contains(n, "APIKEY"):
		return "auth"
	case strings.Contains(n, "SMTP") || strings.Contains(n, "EMAIL") || strings.Contains(n, "MAIL") || strings.Contains(n, "SENDGRID") || strings.Contains(n, "RESEND"):
		return "email"
	case strings.Contains(n, "S3") || strings.Contains(n, "STORAGE") || strings.Contains(n, "BUCKET") || strings.Contains(n, "CLOUDINARY"):
		return "storage"
	case strings.Contains(n, "STRIPE") || strings.Contains(n, "PAYMENT") || strings.Contains(n, "BILLING") || strings.Contains(n, "PAYPAL"):
		return "payment"
	case strings.Contains(n, "API_") || strings.Contains(n, "ENDPOINT") || strings.Contains(n, "BASE_URL") || strings.Contains(n, "WEBHOOK"):
		return "api"
	case strings.HasPrefix(n, "FEATURE_") || strings.HasPrefix(n, "FLAG_") || strings.HasPrefix(n, "ENABLE_") || strings.HasPrefix(n, "DISABLE_"):
		return "feature"
	case strings.HasPrefix(n, "LOG_") || strings.Contains(n, "SENTRY") || strings.Contains(n, "DATADOG"):
		return "logging"
	case strings.Contains(n, "CACHE") || strings.Contains(n, "MEMCACHE"):
		return "cache"
	case strings.Contains(n, "QUEUE") || strings.Contains(n, "RABBIT") || strings.Contains(n, "SQS") || strings.Contains(n, "KAFKA"):
		return "queue"
	case strings.HasPrefix(n, "AWS_") || strings.HasPrefix(n, "GCP_") || strings.HasPrefix(n, "AZURE_") || strings.HasPrefix(n, "VERCEL_"):
		return "cloud"
	}
	return "general"
}

func (e *EnvironmentExtractor) Extract(ctx *projctx.ProjectContext) error {
	vars := map[string]*projctx.EnvVariable{}

	// Strategy 1: .env.example / template / sample (structure only)
	for _, n := range []string{".env.example", ".env.template", ".env.sample"} {
		e.parseEnvExample(filepath.Join(e.Root, n), vars, true)
	}

	// Strategy 2: code scan
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !IsSourceFile(filepath.Base(path)) {
			// also scan docker-compose / Dockerfiles separately
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 2_000_000 {
			return nil
		}
		content := string(data)
		found := map[string]bool{}
		for _, re := range []*regexp.Regexp{reProcessEnvBracket, reProcessEnvDot, reOsEnviron, reOsGetenv, reGoGetenv, reRustEnv, reRubyEnv, reJavaGetenv, reViteEnv} {
			for _, m := range re.FindAllStringSubmatch(content, -1) {
				if len(m) > 1 {
					found[m[1]] = true
				}
			}
		}
		hasDefault := strings.Contains(content, "||") || strings.Contains(content, "??")
		for name := range found {
			if _, ok := vars[name]; !ok {
				vars[name] = &projctx.EnvVariable{Name: name, Category: categorizeEnv(name), Required: !hasDefault}
			}
		}
		// NEXT_PUBLIC_* pattern
		for _, m := range reNextPublic.FindAllString(content, -1) {
			if _, ok := vars[m]; !ok {
				vars[m] = &projctx.EnvVariable{Name: m, Category: "general", Required: false}
			}
		}
		return nil
	})

	// Strategy 3: docker-compose environment
	for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
		p := filepath.Join(e.Root, n)
		if data, err := os.ReadFile(p); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if m := reDockerEnv.FindStringSubmatch(line); len(m) > 1 {
					if _, ok := vars[m[1]]; !ok {
						vars[m[1]] = &projctx.EnvVariable{Name: m[1], Category: categorizeEnv(m[1])}
					}
				}
				// KEY: value under environment:
				t := strings.TrimSpace(line)
				if strings.Contains(t, ":") && len(t) < 120 {
					k := strings.TrimSpace(strings.SplitN(t, ":", 2)[0])
					if len(k) > 2 && k == strings.ToUpper(k) && strings.Contains(k, "_") {
						k = strings.Trim(k, "-\"' ")
						if _, ok := vars[k]; !ok && len(k) < 64 {
							vars[k] = &projctx.EnvVariable{Name: k, Category: categorizeEnv(k)}
						}
					}
				}
			}
		}
	}

	// Strategy 4: Dockerfile ENV
	if data, err := os.ReadFile(filepath.Join(e.Root, "Dockerfile")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if m := reDockerfileEnv.FindStringSubmatch(strings.TrimSpace(line)); len(m) > 1 {
				if _, ok := vars[m[1]]; !ok {
					vars[m[1]] = &projctx.EnvVariable{Name: m[1], Category: categorizeEnv(m[1])}
				}
			}
		}
	}

	list := make([]projctx.EnvVariable, 0, len(vars))
	var required []string
	for _, v := range vars {
		list = append(list, *v)
		if v.Required {
			required = append(required, v.Name)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	sort.Strings(required)
	ctx.Environment = &projctx.EnvironmentContext{Variables: list, Required: required}
	return nil
}

func (e *EnvironmentExtractor) parseEnvExample(path string, vars map[string]*projctx.EnvVariable, required bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lastComment string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			lastComment = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.Contains(name, " ") {
			continue
		}
		if _, ok := vars[name]; !ok {
			vars[name] = &projctx.EnvVariable{
				Name: name, Description: lastComment,
				Category: categorizeEnv(name), Required: required,
			}
		} else if lastComment != "" && vars[name].Description == "" {
			vars[name].Description = lastComment
		}
		lastComment = ""
	}
}
