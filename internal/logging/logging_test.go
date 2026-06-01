package logging

import (
	"bytes"
	"strings"
	"testing"
)

// TestNormalizeLevel verifies canonical level parsing.
func TestNormalizeLevel(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "default", want: "info"},
		{name: "explicit level", raw: "warn", want: "warn"},
		{name: "warning alias", raw: "warning", want: "warn"},
		{name: "off alias", raw: "disabled", want: "off"},
		{name: "invalid", raw: "chatty", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeLevel(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeLevel = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLoggerFiltersByLevel verifies messages below the configured level are not written.
func TestLoggerFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "warn")
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("hidden")
	logger.Warn("shown")

	got := buf.String()
	if strings.Contains(got, "hidden") {
		t.Fatalf("info log should be filtered: %s", got)
	}
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, "shown") {
		t.Fatalf("warn log missing: %s", got)
	}
}

// TestLoggerTraceLevel verifies the custom trace level is emitted with a readable label.
func TestLoggerTraceLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "trace")
	if err != nil {
		t.Fatal(err)
	}

	logger.Trace("wire detail")

	got := buf.String()
	if !strings.Contains(got, "level=TRACE") || !strings.Contains(got, "wire detail") {
		t.Fatalf("trace log missing: %s", got)
	}
}
