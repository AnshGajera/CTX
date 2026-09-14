package extractors

import (
	"regexp"
	"strings"
)

// ignorePattern is a compiled .ctxignore/gitignore-style pattern.
// Supported subset: *, ?, [...], **, trailing / (dir-only),
// leading / (root-anchored), and ! negation. Last match wins.
type ignorePattern struct {
	neg     bool
	dirOnly bool
	regex   *regexp.Regexp
	raw     string
}

func globSegmentToRegex(seg string) string {
	var b strings.Builder
	i := 0
	for i < len(seg) {
		c := seg[i]
		switch c {
		case '*':
			b.WriteString("[^/]*")
			i++
		case '?':
			b.WriteString("[^/]")
			i++
		case '[':
			j := i + 1
			if j < len(seg) && (seg[j] == '!' || seg[j] == '^') {
				j++
			}
			if j < len(seg) && seg[j] == ']' {
				j++
			}
			for j < len(seg) && seg[j] != ']' {
				j++
			}
			if j < len(seg) {
				j++
			}
			cls := seg[i:j]
			// translate [!...] to [^...]
			if strings.HasPrefix(cls, "[!") {
				cls = "[^" + cls[2:]
			}
			b.WriteString(cls)
			i = j
		default:
			b.WriteString(regexp.QuoteMeta(string([]byte{c})))
			i++
		}
	}
	return b.String()
}

func compileIgnorePattern(raw string) *ignorePattern {
	pat := strings.TrimSpace(raw)
	if pat == "" || strings.HasPrefix(pat, "#") {
		return nil
	}
	p := &ignorePattern{raw: raw}
	if strings.HasPrefix(pat, "!") {
		p.neg = true
		pat = strings.TrimSpace(pat[1:])
	}
	if pat == "" {
		return nil
	}
	if strings.HasSuffix(pat, "/") {
		p.dirOnly = true
		pat = strings.TrimSuffix(pat, "/")
	}
	pat = strings.TrimPrefix(pat, "/")
	var body string
	if !strings.Contains(pat, "/") {
		body = "(^|.*/)" + globSegmentToRegex(pat) + "(/.*)?$"
	} else {
		segs := strings.Split(pat, "/")
		parts := make([]string, 0, len(segs))
		for _, s := range segs {
			if s == "**" {
				parts = append(parts, ".*")
			} else {
				parts = append(parts, globSegmentToRegex(s))
			}
		}
		body = "^" + strings.Join(parts, "/") + "(/.*)?$"
	}
	re, err := regexp.Compile(body)
	if err != nil {
		return nil
	}
	p.regex = re
	return p
}

func compileIgnorePatterns(raw []string) []*ignorePattern {
	out := make([]*ignorePattern, 0, len(raw))
	for _, r := range raw {
		if p := compileIgnorePattern(r); p != nil {
			out = append(out, p)
		}
	}
	return out
}

// matchIgnore applies patterns in order; last match wins, ! negates.
func matchIgnore(patterns []*ignorePattern, rel string) bool {
	rel = strings.TrimPrefix(filepathToSlash(rel), "./")
	excluded := false
	for _, p := range patterns {
		if p.regex.MatchString(rel) {
			if p.neg {
				excluded = false
			} else {
				excluded = true
			}
		}
	}
	return excluded
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
