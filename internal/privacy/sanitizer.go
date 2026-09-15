package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"path/filepath"
	"regexp"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// SanitizeConfig controls sanitization.
type SanitizeConfig struct {
	RedactValues    bool
	HashIdentifiers bool
	ExcludePatterns []string
	SensitiveKeys   []string
}

// Sanitizer redacts sensitive data.
type Sanitizer struct {
	config SanitizeConfig
}

var defaultSensitiveKeys = []string{
	"password", "secret", "token", "key", "credential", "private",
	"api_key", "apikey", "auth", "bearer", "access_token", "refresh_token", "session",
}

// NewSanitizer creates a sanitizer with defaults.
func NewSanitizer(config SanitizeConfig) *Sanitizer {
	if len(config.SensitiveKeys) == 0 {
		config.SensitiveKeys = append([]string{}, defaultSensitiveKeys...)
	}
	return &Sanitizer{config: config}
}

func (s *Sanitizer) isSensitiveFieldName(name string) bool {
	n := strings.ToLower(name)
	for _, k := range s.config.SensitiveKeys {
		if strings.Contains(n, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

// ShouldExcludeFile reports if path matches exclude patterns.
func (s *Sanitizer) ShouldExcludeFile(path string) bool {
	slash := filepath.ToSlash(path)
	base := filepath.Base(slash)
	for _, pat := range s.config.ExcludePatterns {
		p := filepath.ToSlash(pat)
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(p, slash); ok {
			return true
		}
		if strings.Contains(slash, strings.Trim(p, "*/")) && strings.Contains(p, "*") {
			return true
		}
	}
	return false
}

// Known secret shapes, redacted wherever they appear in free text.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{10,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['"]?[A-Za-z0-9\-._~+/=]{12,}['"]?`),
}

// tokenRe finds long opaque tokens; entropy decides.
var tokenRe = regexp.MustCompile(`[A-Za-z0-9_\-+=]{20,}`)

const tokenEntropyThreshold = 4.2

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	var freq [256]int
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	var h float64
	n := float64(len(s))
	for _, c := range freq {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// looksSecret reports whether a value looks like a credential.
func looksSecret(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "[REDACTED]" {
		return false
	}
	for _, re := range secretPatterns {
		if re.MatchString(v) {
			return true
		}
	}
	for _, tok := range tokenRe.FindAllString(v, -1) {
		if shannonEntropy(tok) >= tokenEntropyThreshold {
			return true
		}
	}
	return false
}

// redactSecretsInText replaces embedded secrets in free text.
func redactSecretsInText(s string) string {
	if s == "" {
		return s
	}
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	s = tokenRe.ReplaceAllStringFunc(s, func(tok string) string {
		if shannonEntropy(tok) >= tokenEntropyThreshold {
			return "[REDACTED]"
		}
		return tok
	})
	return s
}

// SanitizeContext redacts sensitive defaults/examples in place.
func (s *Sanitizer) SanitizeContext(ctx *projctx.ProjectContext) {
	if ctx == nil {
		return
	}
	redact := s.config.RedactValues
	if ctx.Environment != nil {
		for i := range ctx.Environment.Variables {
			v := &ctx.Environment.Variables[i]
			if s.isSensitiveFieldName(v.Name) {
				if redact {
					v.Default = "[REDACTED]"
					v.Example = "[REDACTED]"
				}
				if s.config.HashIdentifiers {
					v.Name = hashShort(v.Name)
				}
			} else if redact {
				// Generic names can still carry secret values
				// (e.g. key_id default "AKIA...").
				if looksSecret(v.Default) {
					v.Default = "[REDACTED]"
				}
				if looksSecret(v.Example) {
					v.Example = "[REDACTED]"
				}
				v.Description = redactSecretsInText(v.Description)
			}
		}
	}
	if ctx.Database != nil {
		for mi := range ctx.Database.Models {
			for fi := range ctx.Database.Models[mi].Fields {
				f := &ctx.Database.Models[mi].Fields[fi]
				if redact && (s.isSensitiveFieldName(f.Name) || looksSecret(f.Default)) {
					f.Default = "[REDACTED]"
				}
			}
		}
	}
	if !redact {
		return
	}
	// Free-text sections can quote credentials (TODOs, examples,
	// rule descriptions, ADRs). Scan values, never identifiers.
	if ctx.CurrentState != nil {
		for i := range ctx.CurrentState.TODOs {
			ctx.CurrentState.TODOs[i].Text = redactSecretsInText(ctx.CurrentState.TODOs[i].Text)
		}
	}
	if ctx.Patterns != nil {
		for i := range ctx.Patterns.Patterns {
			p := &ctx.Patterns.Patterns[i]
			p.Description = redactSecretsInText(p.Description)
			for j := range p.Examples {
				p.Examples[j] = redactSecretsInText(p.Examples[j])
			}
		}
	}
	if ctx.BusinessRules != nil {
		for i := range ctx.BusinessRules.Rules {
			r := &ctx.BusinessRules.Rules[i]
			r.Description = redactSecretsInText(r.Description)
			for j := range r.Constraints {
				r.Constraints[j] = redactSecretsInText(r.Constraints[j])
			}
		}
	}
	if ctx.Decisions != nil {
		for i := range ctx.Decisions.Decisions {
			d := &ctx.Decisions.Decisions[i]
			d.Title = redactSecretsInText(d.Title)
			d.Description = redactSecretsInText(d.Description)
			d.Reasoning = redactSecretsInText(d.Reasoning)
		}
	}
	if ctx.Architecture != nil {
		ctx.Architecture.Pattern = redactSecretsInText(ctx.Architecture.Pattern)
		for i := range ctx.Architecture.Layers {
			ctx.Architecture.Layers[i].Description = redactSecretsInText(ctx.Architecture.Layers[i].Description)
		}
		for i := range ctx.Architecture.Services {
			ctx.Architecture.Services[i].Description = redactSecretsInText(ctx.Architecture.Services[i].Description)
		}
	}
}

func hashShort(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
