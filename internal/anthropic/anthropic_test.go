package anthropic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestModelIDHelpers verifies Claude Code-visible model alias helpers.
func TestModelIDHelpers(t *testing.T) {
	aliases := Aliases(" claude-opus-custom[1m] ", "claude-sonnet-custom[1m]", "claude-haiku-custom")
	if aliases["opus"] != "claude-opus-custom" || aliases["best"] != "claude-opus-custom" {
		t.Fatalf("opus aliases = %#v", aliases)
	}
	if aliases["sonnet"] != "claude-sonnet-custom" || aliases["haiku"] != "claude-haiku-custom" {
		t.Fatalf("family aliases = %#v", aliases)
	}

	effective := EffectiveAliases(map[string]string{
		" Sonnet ": "claude-sonnet-override[1m]",
		"":         "ignored",
		"empty":    "",
	})
	if effective["sonnet"] != "claude-sonnet-override" {
		t.Fatalf("effective aliases = %#v", effective)
	}
	if effective["opus"] != DefaultOpusModel || effective["haiku"] != DefaultHaikuModel {
		t.Fatalf("default aliases missing = %#v", effective)
	}

	if !DiscoverableByClaudeCode("claude-sonnet-4-6[1m]") || !DiscoverableByClaudeCode("anthropic/custom") {
		t.Fatal("expected Claude-looking model IDs to be discoverable")
	}
	if DiscoverableByClaudeCode("meta/llama") {
		t.Fatal("upstream provider model should not be discoverable")
	}
}

// TestDisplayName verifies display names for known and custom model IDs.
func TestDisplayName(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{model: "claude-sonnet-4-6[1m]", want: "Claude Sonnet 4.6"},
		{model: "provider/claude-opus-4", want: "Claude Opus 4"},
		{model: "claude-haiku-v2-20260602", want: "Claude Haiku"},
		{model: "meta/llama", want: "meta/llama"},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			if got := DisplayName(tc.model); got != tc.want {
				t.Fatalf("DisplayName(%q) = %q, want %q", tc.model, got, tc.want)
			}
		})
	}
}

// TestErrorHelpers verifies Anthropic-style error JSON and upstream error extraction.
func TestErrorHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "invalid_request_error", "bad input")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["type"] != "error" {
		t.Fatalf("body = %#v", body)
	}

	statusTypes := map[int]string{
		http.StatusUnauthorized:        "authentication_error",
		http.StatusForbidden:           "authentication_error",
		http.StatusTooManyRequests:     "rate_limit_error",
		http.StatusNotFound:            "not_found_error",
		http.StatusBadRequest:          "invalid_request_error",
		http.StatusInternalServerError: "api_error",
	}
	for status, want := range statusTypes {
		if got := MapErrorType(status); got != want {
			t.Fatalf("MapErrorType(%d) = %q, want %q", status, got, want)
		}
	}

	if got := ExtractUpstreamError([]byte(`{"error":{"message":"slow down"}}`)); got != "upstream: slow down" {
		t.Fatalf("structured upstream error = %q", got)
	}
	longBody := strings.Repeat("x", 400)
	got := ExtractUpstreamError([]byte(longBody))
	if !strings.HasPrefix(got, "upstream: ") || !strings.HasSuffix(got, "...") || len(got) > len("upstream: ")+303 {
		t.Fatalf("plain upstream error was not safely truncated: %q", got)
	}
}
