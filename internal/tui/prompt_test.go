package tui

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func readerOf(s string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(s))
}

func TestParseSelection(t *testing.T) {
	picks, all, _ := parseSelection("2,5", 6)
	if all || len(picks) != 2 || picks[0] != 1 || picks[1] != 4 {
		t.Fatalf("picks=%v all=%v", picks, all)
	}
	if _, all, _ := parseSelection("", 6); !all {
		t.Fatal("empty must mean all")
	}
	if _, all, _ := parseSelection("all", 6); !all {
		t.Fatal("all must mean all")
	}
	picks, _, _ = parseSelection("0,99,abc,3", 6)
	if len(picks) != 1 || picks[0] != 2 {
		t.Fatalf("invalid entries must be ignored: %v", picks)
	}
}

func TestPromptSectionsToggle(t *testing.T) {
	// Toggle off #2 (api_endpoints); rest stay on.
	out := PromptSections(readerOf("2\n"), io.Discard, DefaultSections())
	found := map[string]bool{}
	for _, k := range out {
		found[k] = true
	}
	if found["api_endpoints"] {
		t.Fatalf("api_endpoints should be off: %v", out)
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 sections: %v", out)
	}
	if out := PromptSections(readerOf("\n"), io.Discard, DefaultSections()); out != nil {
		t.Fatalf("enter must keep all (nil): %v", out)
	}
}

func TestPromptConfirmAndLine(t *testing.T) {
	if !PromptConfirm(readerOf("y\n"), io.Discard, "Q", false) {
		t.Fatal("y should confirm")
	}
	if PromptConfirm(readerOf("\n"), io.Discard, "Q", false) {
		t.Fatal("enter should keep default false")
	}
	if got := PromptLine(readerOf("\n"), io.Discard, "Name", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
	if got := PromptLine(readerOf("myproj\n"), io.Discard, "Name", "fallback"); got != "myproj" {
		t.Fatalf("got %q", got)
	}
}

func TestPromptEditor(t *testing.T) {
	if got := PromptEditor(readerOf("1\n"), io.Discard); got != "cursor" {
		t.Fatalf("got %q", got)
	}
	if got := PromptEditor(readerOf("vscode\n"), io.Discard); got != "vscode" {
		t.Fatalf("got %q", got)
	}
	if got := PromptEditor(readerOf("\n"), io.Discard); got != "none" {
		t.Fatalf("got %q", got)
	}
	if got := PromptEditor(readerOf("bogus\n"), io.Discard); got != "none" {
		t.Fatalf("got %q", got)
	}
}
