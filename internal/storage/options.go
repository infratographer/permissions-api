package storage

import "go.uber.org/zap"

// Option defines a storage engine configuration option.
type Option func(e *engine)

// WithLogger sets the logger for the storage engine.
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(e *engine) {
		e.logger = logger.Named("storage")
	}
}
