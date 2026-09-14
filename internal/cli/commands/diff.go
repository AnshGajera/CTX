package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

// DiffCommand implements `ctx diff [hash1] [hash2]`.
func DiffCommand() *cli.Command {
	return &cli.Command{
		Name:  "diff",
		Usage: "Diff two snapshots (default HEAD~1 HEAD)",
		Flags: []cli.Flag{
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			h1, h2 := "HEAD~1", "HEAD"
			if c.NArg() >= 1 {
				h1 = c.Args().Get(0)
			}
			if c.NArg() >= 2 {
				h2 = c.Args().Get(1)
			}
			s1, err := store.LoadSnapshot(h1)
			if err != nil {
				return fmt.Errorf("resolve %s: %w", h1, err)
			}
			s2, err := store.LoadSnapshot(h2)
			if err != nil {
				return fmt.Errorf("resolve %s: %w", h2, err)
			}
			c1, err := versioning.SnapshotToContext(s1)
			if err != nil {
				return err
			}
			c2, err := versioning.SnapshotToContext(s2)
			if err != nil {
				return err
			}
			d := versioning.ComputeDiff(c1, c2)
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(d)
			}
			green := color.New(color.FgGreen).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			for _, e := range d.AddedEndpoints {
				fmt.Println(green(fmt.Sprintf("+ %s %s (%s)", e.Method, e.Path, e.File)))
			}
			for _, e := range d.RemovedEndpoints {
				fmt.Println(red(fmt.Sprintf("- %s %s (%s)", e.Method, e.Path, e.File)))
			}
			for _, m := range d.AddedModels {
				fmt.Println(green(fmt.Sprintf("+ model %s", m.Name)))
			}
			for _, m := range d.RemovedModels {
				fmt.Println(red(fmt.Sprintf("- model %s", m.Name)))
			}
			for _, v := range d.AddedEnvVars {
				fmt.Println(green(fmt.Sprintf("+ env %s", v.Name)))
			}
			for _, v := range d.RemovedEnvVars {
				fmt.Println(red(fmt.Sprintf("- env %s", v.Name)))
			}
			for _, dep := range d.AddedDeps {
				fmt.Println(green(fmt.Sprintf("+ dep %s@%s", dep.Name, dep.Version)))
			}
			for _, dep := range d.RemovedDeps {
				fmt.Println(red(fmt.Sprintf("- dep %s@%s", dep.Name, dep.Version)))
			}
			fmt.Println("Summary:", d.Summary)
			return nil
		},
	}
}

// LogCommand implements `ctx log`.
func LogCommand() *cli.Command {
	return &cli.Command{
		Name:  "log",
		Usage: "Show snapshot history",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Value: 10, Usage: "max entries"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, _ := os.Getwd()
			store := versioning.NewContextStore(filepath.Join(cwd, ".ctx"))
			snaps, err := store.Log(c.Int("limit"))
			if err != nil {
				return err
			}
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(snaps)
			}
			for _, s := range snaps {
				color.Cyan("%s  %s  %s", shortHash(s.Hash), s.Timestamp.Format("2006-01-02 15:04"), s.Author)
				fmt.Printf("    %s\n", s.Message)
			}
			if len(snaps) == 0 {
				fmt.Println("No snapshots yet. Run ctx extract.")
			}
			return nil
		},
	}
}
