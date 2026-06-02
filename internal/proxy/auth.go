package proxy

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// checkAuth verifies the optional proxy client key from supported auth headers.
func (p *Proxy) checkAuth(r *http.Request) bool {
	if p.cfg.ExpectedClientKey == "" {
		return true
	}
	if k := r.Header.Get("x-api-key"); secureKeyEqual(k, p.cfg.ExpectedClientKey) {
		return true
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		if secureKeyEqual(strings.TrimPrefix(auth, "Bearer "), p.cfg.ExpectedClientKey) {
			return true
		}
	}
	return false
}

// secureKeyEqual compares configured API keys without data-dependent byte comparisons.
func secureKeyEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
