package database

import "go.uber.org/zap"

// Option defines a database configuration option.
type Option func(d *database)

// WithLogger sets the logger for the database.
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(d *database) {
		d.logger = logger.Named("database")
	}
}
