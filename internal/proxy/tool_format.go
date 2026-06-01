package proxy

import "github.com/thomas-illiet/anthropic-proxy/internal/config"

// effectiveToolFormat returns the supported tool conversion mode, defaulting to XML fallback mode.
func effectiveToolFormat(cfg *config.Config) string {
	if cfg != nil && cfg.ToolFormat == config.ToolFormatNative {
		return config.ToolFormatNative
	}
	return config.ToolFormatXML
}
