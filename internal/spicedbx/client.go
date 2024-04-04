// Package spicedbx is the wrapper around spicedb client interaction
package spicedbx

import (
	"context"
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config values for a SpiceDB connection
type Config struct {
	Endpoint   string
	Key        string
	Insecure   bool
	VerifyCA   bool `mapstruct:"verifyca"`
	Prefix     string
	PolicyFile string
}

// NewClient returns a new spicedb/authzed client
func NewClient(cfg Config, enableTracing bool) (*authzed.Client, error) {
	clientOpts := []grpc.DialOption{}

	if cfg.Insecure {
		clientOpts = append(clientOpts,
			grpcutil.WithInsecureBearerToken(cfg.Key),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	} else {
		clientOpts = append(clientOpts,
			grpcutil.WithBearerToken(cfg.Key),
		)

		if cfg.VerifyCA {
			opt, err := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
			if err != nil {
				return nil, fmt.Errorf("failed to load system certificates: %w", err)
			}

			clientOpts = append(clientOpts, opt)
		} else {
			opt, err := grpcutil.WithSystemCerts(grpcutil.SkipVerifyCA)
			if err != nil {
				return nil, fmt.Errorf("failed to load system certificates: %w", err)
			}

			clientOpts = append(clientOpts, opt)
		}
	}

	if enableTracing {
		clientOpts = append(clientOpts,
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		)
	}

	return authzed.NewClient(cfg.Endpoint, clientOpts...)
}

// Healthcheck reads the schema to check if the connection is working
func Healthcheck(client *authzed.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := client.ReadSchema(ctx, &v1.ReadSchemaRequest{}, grpc.WaitForReady(false))
		if err != nil {
			return err
		}

		return nil
	}
}
