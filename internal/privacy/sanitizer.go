package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
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

// SanitizeContext redacts sensitive defaults/examples in place.
func (s *Sanitizer) SanitizeContext(ctx *projctx.ProjectContext) {
	if ctx == nil {
		return
	}
	if ctx.Environment != nil {
		for i := range ctx.Environment.Variables {
			v := &ctx.Environment.Variables[i]
			if s.isSensitiveFieldName(v.Name) {
				if s.config.RedactValues {
					v.Default = "[REDACTED]"
					v.Example = "[REDACTED]"
				}
				if s.config.HashIdentifiers {
					v.Name = hashShort(v.Name)
				}
			}
		}
	}
	if ctx.Database != nil {
		for mi := range ctx.Database.Models {
			for fi := range ctx.Database.Models[mi].Fields {
				f := &ctx.Database.Models[mi].Fields[fi]
				if s.isSensitiveFieldName(f.Name) && s.config.RedactValues {
					f.Default = "[REDACTED]"
				}
			}
		}
	}
}

func hashShort(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
