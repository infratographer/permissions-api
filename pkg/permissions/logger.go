package permissions

import (
	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

var _ retryablehttp.LeveledLogger = (*retryableLogger)(nil)

type retryableLogger struct {
	logger *zap.SugaredLogger
}

// Error implements retryablehttp.LeveledLogger
func (l *retryableLogger) Error(msg string, keysAndValues ...any) {
	l.logger.Errorw(msg, keysAndValues...)
}

// Info implements retryablehttp.LeveledLogger
func (l *retryableLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Infow(msg, keysAndValues...)
}

// Debug implements retryablehttp.LeveledLogger
func (l *retryableLogger) Debug(msg string, keysAndValues ...any) {
	l.logger.Debugw(msg, keysAndValues...)
}

// Warn implements retryablehttp.LeveledLogger
func (l *retryableLogger) Warn(msg string, keysAndValues ...any) {
	l.logger.Warnw(msg, keysAndValues...)
}
