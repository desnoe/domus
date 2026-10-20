package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Env represents the target deployment environment
type Env string

const (
	EnvProd    Env = "prod"
	EnvPreview Env = "preview"
	EnvPR      Env = "pr"
)

// Config encapsulates environment-specific scoping for folders and dashboard identifiers
type Config struct {
	Env        Env
	TargetSlug string
	FolderUID  string
	FolderName string
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9-]+`)

// NewConfig initializes environment configuration with sanitized slugs
func NewConfig(env Env, slug string, folder string) Config {
	c := Config{
		Env:        env,
		TargetSlug: slug,
	}
	if c.TargetSlug == "" {
		c.TargetSlug = "local"
	}
	c.TargetSlug = SanitizeSlug(c.TargetSlug)

	switch env {
	case EnvPreview:
		c.FolderUID = fmt.Sprintf("domus-prev-%s", c.TargetSlug)
		c.FolderName = fmt.Sprintf("domus (Preview %s)", c.TargetSlug)
	case EnvPR:
		c.FolderUID = fmt.Sprintf("domus-pr-%s", c.TargetSlug)
		c.FolderName = fmt.Sprintf("domus (PR #%s)", c.TargetSlug)
	default:
		c.FolderUID = "domus"
		c.FolderName = "domus"
	}

	if folder != "" {
		c.FolderUID = folder
		c.FolderName = folder
	}
	return c
}

// SanitizeSlug converts branch or PR identifiers into URL and UID safe strings
func SanitizeSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 14 {
		h := sha256.Sum256([]byte(s))
		hashStr := hex.EncodeToString(h[:])[:5]
		prefix := s
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		s = fmt.Sprintf("%s-%s", strings.Trim(prefix, "-"), hashStr)
	}
	return s
}

// DashboardUID derives a deterministic UID based on environment and target slug (max 40 chars)
func (c Config) DashboardUID(baseUID string) string {
	switch c.Env {
	case EnvPreview:
		uid := fmt.Sprintf("%s-p-%s", baseUID, c.TargetSlug)
		if len(uid) > 40 {
			uid = uid[:40]
		}
		return uid
	case EnvPR:
		uid := fmt.Sprintf("%s-pr-%s", baseUID, c.TargetSlug)
		if len(uid) > 40 {
			uid = uid[:40]
		}
		return uid
	default:
		return baseUID
	}
}

// DashboardTitle derives a human-readable title prefixed with environment indicator
func (c Config) DashboardTitle(baseTitle string) string {
	switch c.Env {
	case EnvPreview:
		return fmt.Sprintf("[Preview %s] domus / %s", c.TargetSlug, baseTitle)
	case EnvPR:
		return fmt.Sprintf("[PR #%s] domus / %s", c.TargetSlug, baseTitle)
	default:
		return fmt.Sprintf("domus / %s", baseTitle)
	}
}
