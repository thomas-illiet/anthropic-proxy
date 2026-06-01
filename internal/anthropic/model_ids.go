package anthropic

import "strings"

const (
	DefaultOpusModel   = "claude-opus-4-8"
	DefaultSonnetModel = "claude-sonnet-4-6"
	DefaultHaikuModel  = "claude-haiku-4-5"

	UnknownCreatedAt = "1970-01-01T00:00:00Z"
)

var FamilyAliasOrder = []string{"sonnet", "opus", "haiku"}

// Aliases builds Claude Code family aliases from configured Anthropic-visible model IDs.
func Aliases(opusModel, sonnetModel, haikuModel string) map[string]string {
	opusModel = StripContextSuffix(opusModel)
	sonnetModel = StripContextSuffix(sonnetModel)
	haikuModel = StripContextSuffix(haikuModel)
	return map[string]string{
		"sonnet": sonnetModel,
		"opus":   opusModel,
		"haiku":  haikuModel,
		"best":   opusModel,
	}
}

// DefaultAliases returns the built-in Claude Code family alias map.
func DefaultAliases() map[string]string {
	return Aliases(DefaultOpusModel, DefaultSonnetModel, DefaultHaikuModel)
}

// EffectiveAliases merges configured aliases over the built-in defaults.
func EffectiveAliases(aliases map[string]string) map[string]string {
	out := DefaultAliases()
	for key, value := range aliases {
		key = strings.ToLower(strings.TrimSpace(key))
		value = StripContextSuffix(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// StripContextSuffix removes Claude Code context-window suffixes from a model ID.
func StripContextSuffix(model string) string {
	model = strings.TrimSpace(model)
	if strings.HasSuffix(model, "[1m]") {
		return strings.TrimSpace(strings.TrimSuffix(model, "[1m]"))
	}
	return model
}

// DiscoverableByClaudeCode reports whether Claude Code should accept a discovered model ID.
func DiscoverableByClaudeCode(model string) bool {
	model = StripContextSuffix(model)
	return strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "anthropic")
}

// DisplayName derives a human-readable Claude model name from an Anthropic-visible model ID.
func DisplayName(model string) string {
	original := StripContextSuffix(model)
	base := original
	if idx := strings.Index(base, "claude-"); idx >= 0 {
		base = base[idx:]
	}

	parts := strings.Split(base, "-")
	if len(parts) < 2 || parts[0] != "claude" {
		return original
	}

	family := title(parts[1])
	version := displayVersion(parts[2:])
	if version == "" {
		return "Claude " + family
	}
	return "Claude " + family + " " + version
}

// displayVersion converts numeric model ID segments into a dotted version label.
func displayVersion(parts []string) string {
	nums := []string{}
	for _, part := range parts {
		part = strings.TrimSuffix(part, ":0")
		if part == "" || strings.HasPrefix(part, "v") {
			continue
		}
		if !allDigits(part) {
			continue
		}
		if len(part) > 2 {
			continue
		}
		nums = append(nums, part)
		if len(nums) == 2 {
			return nums[0] + "." + nums[1]
		}
	}
	if len(nums) == 1 {
		return nums[0]
	}
	return ""
}

// title uppercases the first byte of a lowercase ASCII label.
func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// allDigits reports whether a string is non-empty and contains only ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
