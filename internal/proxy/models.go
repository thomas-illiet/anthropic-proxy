package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/thomas-illiet/anthropic-proxy/internal/anthropic"
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
)

const defaultModelPageLimit = 20
const maxModelPageLimit = 1000

// handleModels serves Anthropic-compatible model discovery for Claude Code gateways.
func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	if !p.checkAuth(r) {
		p.warnf("models authentication failed client=%s", getClientIP(r))
		anthropic.WriteError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return
	}

	out, err := paginateModels(configuredModels(p.cfg), r.URL.Query())
	if err != nil {
		p.warnf("models pagination failed: %v", err)
		anthropic.WriteError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// configuredModels returns the Anthropic-visible models exposed by proxy configuration.
func configuredModels(cfg *config.Config) []anthropic.ModelInfo {
	seen := map[string]bool{}
	models := []anthropic.ModelInfo{}
	// add appends a normalized model once while preserving catalog order.
	add := func(id string) {
		id = anthropic.StripContextSuffix(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		models = append(models, anthropic.ModelInfo{
			ID:          id,
			Type:        "model",
			DisplayName: modelDisplayName(cfg, id),
			CreatedAt:   anthropic.UnknownCreatedAt,
		})
	}

	aliases := effectiveModelAliases(cfg)
	for _, alias := range anthropic.FamilyAliasOrder {
		add(aliases[alias])
	}

	keys := make([]string, 0, len(cfg.ModelMap))
	for k := range cfg.ModelMap {
		k = anthropic.StripContextSuffix(k)
		if anthropic.DiscoverableByClaudeCode(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		add(k)
	}

	defaultModel := anthropic.StripContextSuffix(cfg.DefaultModel)
	if anthropic.DiscoverableByClaudeCode(defaultModel) {
		add(defaultModel)
	}

	return models
}

// paginateModels applies Anthropic-style cursor pagination to a model list.
func paginateModels(models []anthropic.ModelInfo, query url.Values) (anthropic.ModelList, error) {
	limit, err := modelPageLimit(query.Get("limit"))
	if err != nil {
		return anthropic.ModelList{}, err
	}

	start, end := 0, len(models)
	if afterID := query.Get("after_id"); afterID != "" {
		idx := modelIndex(models, afterID)
		if idx < 0 {
			return anthropic.ModelList{}, fmt.Errorf("unknown after_id %q", afterID)
		}
		start = idx + 1
	}
	if beforeID := query.Get("before_id"); beforeID != "" {
		idx := modelIndex(models, beforeID)
		if idx < 0 {
			return anthropic.ModelList{}, fmt.Errorf("unknown before_id %q", beforeID)
		}
		end = idx
	}
	if start > end {
		start = end
	}

	hasMore := false
	if end-start > limit {
		end = start + limit
		hasMore = true
	}

	page := append([]anthropic.ModelInfo(nil), models[start:end]...)
	out := anthropic.ModelList{
		Data:    page,
		HasMore: hasMore,
	}
	if len(page) > 0 {
		out.FirstID = page[0].ID
		out.LastID = page[len(page)-1].ID
	}
	return out, nil
}

// modelPageLimit parses and validates the list-models page size.
func modelPageLimit(raw string) (int, error) {
	if raw == "" {
		return defaultModelPageLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxModelPageLimit {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d", maxModelPageLimit)
	}
	return n, nil
}

// modelIndex returns the index of a model ID in a list, or -1 when absent.
func modelIndex(models []anthropic.ModelInfo, id string) int {
	id = anthropic.StripContextSuffix(id)
	for i, model := range models {
		if model.ID == id {
			return i
		}
	}
	return -1
}

// modelDisplayName returns a configured or derived display name for a model ID.
func modelDisplayName(cfg *config.Config, id string) string {
	if cfg.ModelDisplayNames != nil {
		if name := cfg.ModelDisplayNames[id]; name != "" {
			return name
		}
	}
	return anthropic.DisplayName(id)
}

// effectiveModelAliases returns model aliases with default family aliases filled in.
func effectiveModelAliases(cfg *config.Config) map[string]string {
	return anthropic.EffectiveAliases(cfg.ModelAliases)
}
