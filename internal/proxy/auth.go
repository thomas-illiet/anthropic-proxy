package proxy

import (
	"net/http"
	"strings"
)

// checkAuth verifies the optional proxy client key from supported auth headers.
func (p *Proxy) checkAuth(r *http.Request) bool {
	if p.cfg.ExpectedClientKey == "" {
		return true
	}
	if k := r.Header.Get("x-api-key"); k == p.cfg.ExpectedClientKey {
		return true
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.TrimPrefix(auth, "Bearer ") == p.cfg.ExpectedClientKey {
			return true
		}
	}
	return false
}
