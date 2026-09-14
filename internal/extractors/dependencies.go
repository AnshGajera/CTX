package extractors

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/ctxdev/ctx/internal/context"
)

// DependencyExtractor parses manifests for multiple languages.
type DependencyExtractor struct{ Base }

func NewDependencies(root string) *DependencyExtractor {
	return &DependencyExtractor{Base: NewBase(root)}
}

func (e *DependencyExtractor) Name() string { return "dependencies" }

var purposeMap = map[string]string{
	"next": "Fullstack React framework", "react": "UI component library",
	"react-dom": "UI rendering", "express": "HTTP server framework",
	"fastify": "HTTP server framework", "hono": "HTTP server framework",
	"@nestjs/core": "Backend framework", "prisma": "Database ORM",
	"@prisma/client": "Database ORM client", "stripe": "Payment processing",
	"@clerk/nextjs": "Authentication provider", "zod": "Runtime type validation",
	"tailwindcss": "Utility-first CSS framework", "axios": "HTTP client",
	"lodash": "Utility functions", "typescript": "Type safety",
	"vitest": "Unit testing", "jest": "Unit testing", "playwright": "E2E testing",
	"cypress": "E2E testing", "eslint": "Linting", "prettier": "Formatting",
	"mongoose": "MongoDB ODM", "pg": "Postgres driver", "mysql2": "MySQL driver",
	"redis": "Cache client", "ioredis": "Cache client", "typeorm": "Database ORM",
	"drizzle-orm": "Database ORM", "@supabase/supabase-js": "Backend platform",
	"next-auth": "Authentication", "jsonwebtoken": "JWT handling",
	"bcrypt": "Password hashing", "bcryptjs": "Password hashing",
	"nodemailer": "Email sending", "resend": "Email API", "@sendgrid/mail": "Email API",
	"aws-sdk": "AWS SDK", "@aws-sdk/client-s3": "S3 storage",
	"cloudinary": "Media storage", "multer": "File upload",
	"winston": "Logging", "pino": "Logging", "morgan": "HTTP logging",
	"sentry": "Error monitoring", "@sentry/nextjs": "Error monitoring",
	"bull": "Job queue", "bullmq": "Job queue", "kafkajs": "Kafka client",
	"amqplib": "RabbitMQ client", "socket.io": "WebSockets",
	"graphql": "GraphQL", "apollo-server": "GraphQL server",
	"trpc": "Typesafe API", "@tanstack/react-query": "Data fetching",
	"swr": "Data fetching", "redux": "State management", "zustand": "State management",
	"recoil": "State management", "@reduxjs/toolkit": "State management",
	"framer-motion": "Animation", "chart.js": "Charts", "recharts": "Charts",
	"d3": "Data visualization", "dayjs": "Date utils", "date-fns": "Date utils",
	"moment": "Date utils", "yup": "Validation", "joi": "Validation",
	"express-validator": "Validation", "cors": "CORS middleware", "helmet": "Security headers",
	"dotenv": "Env loading", "commander": "CLI args", "inquirer": "CLI prompts",
	"chalk": "Terminal colors", "ora": "CLI spinners", "vite": "Build tool",
	"webpack": "Bundler", "esbuild": "Bundler", "turbo": "Monorepo builds",
	"husky": "Git hooks", "lint-staged": "Staged linting",
	"@testing-library/react": "Component testing", "@testing-library/jest-dom": "Test matchers",
	"supertest": "API testing", "mocha": "Test runner", "chai": "Assertions",
	"openai": "LLM API", "langchain": "LLM orchestration",
	"@langchain/openai": "LLM provider", "ai": "Vercel AI SDK",
	"sharp": "Image processing", "pdfkit": "PDF generation",
	"exceljs": "Spreadsheet", "csv-parse": "CSV parsing",
	"cheerio": "HTML scraping", "puppeteer": "Browser automation",
	"firebase": "BaaS", "firebase-admin": "BaaS admin",
	"auth0": "Authentication", "@auth0/nextjs-auth0": "Authentication",
	"passport": "Authentication", "oauth": "OAuth",
	"cron": "Scheduling", "node-cron": "Scheduling",
	"uuid": "ID generation", "nanoid": "ID generation",
	"clsx": "Classnames", "classnames": "Classnames",
	"@radix-ui/react-slot": "UI primitives", "shadcn": "UI components",
	"@mui/material": "UI kit", "@chakra-ui/react": "UI kit", "antd": "UI kit",
	"styled-components": "CSS-in-JS", "@emotion/react": "CSS-in-JS",
}

var categoryMap = map[string]string{
	"next": "framework", "react": "ui", "express": "framework", "prisma": "database",
	"stripe": "payment", "zod": "validation", "tailwindcss": "styling",
	"jest": "testing", "vitest": "testing", "vite": "build", "winston": "logging",
}

var criticalDeps = map[string]bool{
	"next": true, "react": true, "express": true, "fastify": true,
	"prisma": true, "@prisma/client": true, "mongoose": true, "pg": true,
	"typescript": true, "zod": true, "stripe": true,
}

func inferPurpose(name string) string {
	if p, ok := purposeMap[name]; ok {
		return p
	}
	// generic heuristics
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "auth") || strings.Contains(l, "clerk"):
		return "Authentication"
	case strings.Contains(l, "db") || strings.Contains(l, "sql") || strings.Contains(l, "mongo"):
		return "Database"
	case strings.Contains(l, "test") || strings.Contains(l, "jest") || strings.Contains(l, "vitest"):
		return "Testing"
	case strings.Contains(l, "lint") || strings.Contains(l, "prettier"):
		return "Code quality"
	}
	return "Utility library"
}

func inferCategory(name string) string {
	if c, ok := categoryMap[name]; ok {
		return c
	}
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "auth") || strings.Contains(l, "jwt") || strings.Contains(l, "passport"):
		return "auth"
	case strings.Contains(l, "mongo") || strings.Contains(l, "prisma") || strings.Contains(l, "typeorm") || strings.Contains(l, "drizzle") || strings.Contains(l, "pg") || strings.Contains(l, "mysql") || strings.Contains(l, "redis"):
		return "database"
	case strings.Contains(l, "stripe") || strings.Contains(l, "payment") || strings.Contains(l, "paypal"):
		return "payment"
	case strings.Contains(l, "mail") || strings.Contains(l, "sendgrid") || strings.Contains(l, "resend") || strings.Contains(l, "nodemailer"):
		return "email"
	case strings.Contains(l, "s3") || strings.Contains(l, "storage") || strings.Contains(l, "cloudinary") || strings.Contains(l, "bucket"):
		return "storage"
	case strings.Contains(l, "test") || strings.Contains(l, "jest") || strings.Contains(l, "vitest") || strings.Contains(l, "cypress") || strings.Contains(l, "playwright"):
		return "testing"
	case strings.Contains(l, "eslint") || strings.Contains(l, "vite") || strings.Contains(l, "webpack") || strings.Contains(l, "esbuild") || strings.Contains(l, "turbo"):
		return "build"
	case strings.Contains(l, "winston") || strings.Contains(l, "pino") || strings.Contains(l, "sentry"):
		return "logging"
	case strings.Contains(l, "react") || strings.Contains(l, "vue") || strings.Contains(l, "tailwind") || strings.Contains(l, "mui") || strings.Contains(l, "chakra"):
		return "ui"
	case strings.Contains(l, "axios") || strings.Contains(l, "fetch") || strings.Contains(l, "graphql") || strings.Contains(l, "trpc"):
		return "api"
	case strings.Contains(l, "bull") || strings.Contains(l, "kafka") || strings.Contains(l, "rabbit") || strings.Contains(l, "sqs"):
		return "queue"
	case strings.Contains(l, "redis") || strings.Contains(l, "memcache"):
		return "cache"
	}
	return "general"
}

func (e *DependencyExtractor) Extract(ctx *projctx.ProjectContext) error {
	dc := &projctx.DependencyContext{}
	e.extractNode(dc)
	e.extractGo(dc)
	e.extractPython(dc)
	e.extractRust(dc)
	ctx.Dependencies = dc
	return nil
}

func (e *DependencyExtractor) extractNode(dc *projctx.DependencyContext) {
	data, err := os.ReadFile(filepath.Join(e.Root, "package.json"))
	if err != nil {
		return
	}
	var pj struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pj); err != nil {
		return
	}
	for name, ver := range pj.Dependencies {
		dc.Direct = append(dc.Direct, projctx.Dependency{
			Name: name, Version: ver, Purpose: inferPurpose(name),
			Category: inferCategory(name), Critical: criticalDeps[name],
		})
	}
	for name, ver := range pj.DevDependencies {
		dc.Dev = append(dc.Dev, projctx.Dependency{
			Name: name, Version: ver, Purpose: inferPurpose(name),
			Category: inferCategory(name), Critical: false,
		})
	}
}

func (e *DependencyExtractor) extractGo(dc *projctx.DependencyContext) {
	f, err := os.Open(filepath.Join(e.Root, "go.mod"))
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inRequire := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire || strings.HasPrefix(line, "require ") {
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) >= 2 {
				name := fields[0]
				ver := fields[1]
				dc.Direct = append(dc.Direct, projctx.Dependency{
					Name: name, Version: ver, Purpose: "Go module",
					Category: "general", Critical: false,
				})
			}
		}
	}
}

func (e *DependencyExtractor) extractPython(dc *projctx.DependencyContext) {
	if data, err := os.ReadFile(filepath.Join(e.Root, "requirements.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			name := line
			ver := ""
			for _, sep := range []string{"==", ">=", "~=", "!="} {
				if strings.Contains(line, sep) {
					parts := strings.SplitN(line, sep, 2)
					name = strings.TrimSpace(parts[0])
					ver = strings.TrimSpace(parts[1])
					break
				}
			}
			dc.Direct = append(dc.Direct, projctx.Dependency{
				Name: name, Version: ver, Purpose: inferPurpose(name),
				Category: inferCategory(name),
			})
		}
	}
	if data, err := os.ReadFile(filepath.Join(e.Root, "pyproject.toml")); err == nil {
		s := string(data)
		// naive [project] dependencies extraction
		inDeps := false
		for _, line := range strings.Split(s, "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "dependencies") {
				inDeps = true
				continue
			}
			if inDeps {
				if strings.HasPrefix(t, "]") || (strings.HasPrefix(t, "[") && !strings.HasPrefix(t, "\"")) {
					break
				}
				name := strings.Trim(t, "\", ")
				if name != "" && !strings.HasPrefix(name, "#") {
					dc.Direct = append(dc.Direct, projctx.Dependency{Name: name, Purpose: inferPurpose(name), Category: inferCategory(name)})
				}
			}
		}
	}
}

func (e *DependencyExtractor) extractRust(dc *projctx.DependencyContext) {
	data, err := os.ReadFile(filepath.Join(e.Root, "Cargo.toml"))
	if err != nil {
		return
	}
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "[dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(t, "[") && inDeps {
			break
		}
		if inDeps && strings.Contains(t, "=") {
			name := strings.TrimSpace(strings.SplitN(t, "=", 2)[0])
			dc.Direct = append(dc.Direct, projctx.Dependency{Name: name, Purpose: inferPurpose(name), Category: inferCategory(name)})
		}
	}
}
