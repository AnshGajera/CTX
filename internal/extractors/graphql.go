package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// GraphQLExtractor extracts GraphQL schema, queries, mutations, and subscriptions.
type GraphQLExtractor struct {
	Base
}

// NewGraphQL creates a GraphQL extractor.
func NewGraphQL(root string) *GraphQLExtractor {
	return &GraphQLExtractor{Base: NewBase(root)}
}

func (e *GraphQLExtractor) Name() string { return "graphql" }

var (
	gqlTypeRe         = regexp.MustCompile(`(?i)^\s*type\s+(\w+)\s*(\{|@|implements)`)
	gqlFieldRe        = regexp.MustCompile(`^\s+(\w+)\s*(?:\([^)]*\))?\s*:\s*(.+)`)
	gqlQueryRe        = regexp.MustCompile(`(?i)^\s*type\s+Query\s*\{`)
	gqlMutationRe     = regexp.MustCompile(`(?i)^\s*type\s+Mutation\s*\{`)
	gqlSubscriptionRe = regexp.MustCompile(`(?i)^\s*type\s+Subscription\s*\{`)
	gqlInputRe        = regexp.MustCompile(`(?i)^\s*input\s+(\w+)\s*\{`)
	gqlEnumRe         = regexp.MustCompile(`(?i)^\s*enum\s+(\w+)\s*\{`)
	gqlResolverRe     = regexp.MustCompile(`(?i)(Query|Mutation|Subscription)\s*:\s*\{`)
)

func (e *GraphQLExtractor) Extract(ctx *projctx.ProjectContext) error {
	var endpoints []projctx.APIEndpoint

	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		ext := filepath.Ext(rel)
		if ext != ".graphql" && ext != ".gql" && ext != ".graphqls" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		inBlock := ""     // "query", "mutation", "subscription", ""
		braceDepth := 0

		for sc.Scan() {
			line := sc.Text()

			// Detect query/mutation/subscription type blocks
			if gqlQueryRe.MatchString(line) {
				inBlock = "query"
				braceDepth = 1
				continue
			}
			if gqlMutationRe.MatchString(line) {
				inBlock = "mutation"
				braceDepth = 1
				continue
			}
			if gqlSubscriptionRe.MatchString(line) {
				inBlock = "subscription"
				braceDepth = 1
				continue
			}

			if inBlock != "" {
				braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
				if braceDepth <= 0 {
					inBlock = ""
					continue
				}

				if m := gqlFieldRe.FindStringSubmatch(line); len(m) >= 3 {
					method := "QUERY"
					if inBlock == "mutation" {
						method = "MUTATION"
					} else if inBlock == "subscription" {
						method = "SUBSCRIPTION"
					}

					endpoints = append(endpoints, projctx.APIEndpoint{
						Method:  method,
						Path:    m[1],
						Handler: strings.TrimSpace(m[2]),
						File:    rel,
					})
				}
			}
		}
		return nil
	})

	// Also scan .ts/.js files for resolver definitions
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !IsSourceFile(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 512*1024 {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "resolver") && !strings.Contains(content, "Resolver") {
			return nil
		}

		// Find resolver method patterns
		resolverMethodRe := regexp.MustCompile(`(?m)^\s*(?:async\s+)?(\w+)\s*(?:\(|:\s*async)`)
		if gqlResolverRe.MatchString(content) {
			lines := strings.Split(content, "\n")
			inResolver := ""
			for _, line := range lines {
				if strings.Contains(line, "Query") && strings.Contains(line, "{") {
					inResolver = "QUERY"
				} else if strings.Contains(line, "Mutation") && strings.Contains(line, "{") {
					inResolver = "MUTATION"
				} else if strings.Contains(line, "Subscription") && strings.Contains(line, "{") {
					inResolver = "SUBSCRIPTION"
				}
				if inResolver != "" {
					if m := resolverMethodRe.FindStringSubmatch(line); len(m) >= 2 {
						name := m[1]
						if name != "Query" && name != "Mutation" && name != "Subscription" &&
							name != "async" && name != "return" && name != "const" {
							endpoints = append(endpoints, projctx.APIEndpoint{
								Method:  inResolver,
								Path:    name,
								Handler: name,
								File:    rel,
							})
						}
					}
				}
			}
		}
		return nil
	})

	if len(endpoints) > 0 {
		if ctx.APIs == nil {
			ctx.APIs = &projctx.APIContext{}
		}
		ctx.APIs.Endpoints = append(ctx.APIs.Endpoints, endpoints...)
	}

	return nil
}
