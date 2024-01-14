package cmd

import (
	"errors"

	"github.com/nats-io/nats.go"
	"go.infratographer.com/x/events"

	"go.infratographer.com/permissions-api/internal/config"
)

var (
	errInvalidSource = errors.New("events source must be a NATS connection")
)

func initializeKV(cfg config.EventsConfig, eventsConn events.Connection) (nats.KeyValue, error) {
	// While in theory the events package supports any kind of broker, in practice we only
	// support NATS.
	natsConn, ok := eventsConn.Source().(*nats.Conn)
	if !ok {
		return nil, errInvalidSource
	}

	js, err := natsConn.JetStream()
	if err != nil {
		return nil, err
	}

	return js.KeyValue(cfg.ZedTokenBucket)
}
