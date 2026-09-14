package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AnshGajera/CTX/internal/ai"
	"github.com/AnshGajera/CTX/internal/config"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/urfave/cli/v2"
)

// SearchCommand implements `ctx search` (semantic hybrid retrieval).
func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "Semantic search over project context",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "top", Value: 5, Usage: "top K results"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return fmt.Errorf("usage: ctx search <query>")
			}
			query := c.Args().Get(0)
			cwd, _ := os.Getwd()
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return err
			}
			chunks := ai.NewChunker().Chunk(ctx)
			cfg, _ := config.Load(config.ProjectConfigPath(cwd))
			ranked := ai.HybridSearch(cfg.Core.MLURL, query, chunks, c.Int("top"))
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(ranked)
			}
			for i, r := range ranked {
				fmt.Printf("%d. [%s] %.3f %s\n   %s\n", i+1, r.Chunk.Kind, r.Score, r.Chunk.ID, r.Chunk.Snippet)
			}
			if len(ranked) == 0 {
				fmt.Println("No results")
			}
			return nil
		},
	}
}

// EvalCommand implements `ctx eval` (retrieval hit@k on bundled cases).
func EvalCommand() *cli.Command {
	return &cli.Command{
		Name:  "eval",
		Usage: "Evaluate retrieval quality (hit@k)",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "k", Value: 5, Usage: "hit@k"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			ctx, err := projctx.LoadContext(cwd)
			if err != nil {
				return err
			}
			chunks := ai.NewChunker().Chunk(ctx)
			k := c.Int("k")
			// generic self-check cases derived from context
			type evalCase struct {
				query  string
				expect string // substring expected in top-k
			}
			var cases []evalCase
			if ctx.APIs != nil {
				for i, e := range ctx.APIs.Endpoints {
					if i >= 5 {
						break
					}
					cases = append(cases, evalCase{query: e.Method + " " + e.Path, expect: e.Path})
				}
			}
			if ctx.Database != nil {
				for i, m := range ctx.Database.Models {
					if i >= 5 {
						break
					}
					cases = append(cases, evalCase{query: "model " + m.Name, expect: m.Name})
				}
			}
			if len(cases) == 0 {
				fmt.Println("Not enough context to evaluate; run ctx extract on a real project first.")
				return nil
			}
			hits := 0
			for _, ec := range cases {
				ranked := ai.RankTFIDF(chunks, ec.query, k)
				for _, r := range ranked {
					if containsFold(r.Chunk.Text, ec.expect) {
						hits++
						break
					}
				}
			}
			score := float64(hits) / float64(len(cases))
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"k": k, "hit_at_k": score, "hits": hits, "total": len(cases),
				})
			}
			fmt.Printf("hit@%d: %.2f (%d/%d)\n", k, score, hits, len(cases))
			return nil
		},
	}
}

func containsFold(hay, needle string) bool {
	return len(hay) >= 0 && len(needle) > 0 &&
		containsLower(hay, needle)
}

func containsLower(hay, needle string) bool {
	h, n := toLower(hay), toLower(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		b[i] = ch
	}
	return string(b)
}
