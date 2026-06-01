package proxy

import (
	"github.com/thomas-illiet/anthropic-proxy/internal/config"
	"github.com/thomas-illiet/anthropic-proxy/internal/logging"
)

// configLogLevel returns the configured log level or the default used for manually built configs.
func configLogLevel(cfg *config.Config) string {
	if cfg != nil && cfg.LogLevel != "" {
		return cfg.LogLevel
	}
	return logging.DefaultLevel
}

// tracef writes a formatted trace log line when the configured level allows it.
func (p *Proxy) tracef(format string, args ...any) {
	p.logger.Tracef(format, args...)
}

// debugf writes a formatted debug log line when the configured level allows it.
func (p *Proxy) debugf(format string, args ...any) {
	p.logger.Debugf(format, args...)
}

// warnf writes a formatted warning log line when the configured level allows it.
func (p *Proxy) warnf(format string, args ...any) {
	p.logger.Warnf(format, args...)
}

// errorf writes a formatted error log line when the configured level allows it.
func (p *Proxy) errorf(format string, args ...any) {
	p.logger.Errorf(format, args...)
}
