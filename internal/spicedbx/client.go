package spicedbx

import (
	"context"

	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config values for a SpiceDB connection
type Config struct {
	Endpoint string
	Key      string
	Insecure bool
	VerifyCA bool `mapstruct:"verifyca"`
	Prefix   string
}

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
			clientOpts = append(clientOpts, grpcutil.WithSystemCerts(grpcutil.VerifyCA))
		} else {
			clientOpts = append(clientOpts, grpcutil.WithSystemCerts(grpcutil.SkipVerifyCA))
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

func Healthcheck(client *authzed.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return nil
	}
}
