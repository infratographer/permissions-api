package pubsubx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.infratographer.com/permissions-api/internal/query"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"gocloud.dev/pubsub"
	"gocloud.dev/pubsub/natspubsub"
)

var (
	tracer = otel.Tracer("go.infratographer.com/x/pubsubx")
)

type Message struct {
	SubjectURN            string                 `json:"subject_urn"`
	EventType             string                 `json:"event_type"`
	AdditionalSubjectURNs []string               `json:"additional_subjects"`
	ActorURN              string                 `json:"actor_urn"`
	Source                string                 `json:"source"`
	Timestamp             time.Time              `json:"timestamp"`
	SubjectFields         map[string]string      `json:"fields"`
	AdditionalData        map[string]interface{} `json:"additional_data"`
}

type Subscription struct {
	sub    *pubsub.Subscription
	logger *zap.SugaredLogger
}

type SubscriptionOption func(*SubscriptionOptions) error

// Options can be used to create a customized connection.
type SubscriptionOptions struct {
	Queue string
}

// type Publisher struct {
// 	topic *pubsub.Topic
// 	logger *zap.SugaredLogger
// }

func NewSubscription(ctx context.Context, psURL string, logger *zap.SugaredLogger, opts ...SubscriptionOption) (*Subscription, error) {
	u, err := url.Parse(psURL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "nats":
		fmt.Println("Start a NATs Sub")
	default:
		return nil, errors.New("currently only NATs is supported for pubsub")
	}

	natsConn, err := nats.Connect(psURL)
	if err != nil {
		return nil, err
	}
	// defer natsConn.Drain()

	// natsConn.Close()

	sub, err := natspubsub.OpenSubscription(
		natsConn,
		"com.infratographer.events.*.*",
		&natspubsub.SubscriptionOptions{Queue: "permissionsapi"})
	if err != nil {
		return nil, err
	}

	logger = logger.Named("worker")

	return &Subscription{sub: sub, logger: logger}, nil
}

func (s *Subscription) StartListening(ctx context.Context, st *query.Stores) error {
	// sub, err := pubsub.OpenSubscription(ctx, "nats://com.infratographer.events.*")
	// if err != nil {
	// 	return fmt.Errorf("could not open topic subscription: %v", err)
	// }
	//nolint:errcheck // TODO: figure out how to handle this error
	defer s.sub.Shutdown(ctx)

	fmt.Println("Starting to listen for a messages")

	// Loop on received messages.
	for {
		if err := s.Receive(ctx, st); err != nil {
			return err
		}
	}
}

// func NewPublisher(ctx context.Context, psURL string, logger *zap.SugaredLogger, opts ...SubscriptionOption) (*Publisher, error) {
// 	u, err := url.Parse(psURL)
// 	if err != nil {
// 		return nil, err
// 	}

// 	switch u.Scheme {
// 	case "nats":
// 		fmt.Println("Start a NATs Sub")
// 	default:
// 		return nil, errors.New("currently only NATs is supported for pubsub")
// 	}

// 	natsConn, err := nats.Connect(psURL)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// defer natsConn.Drain()

// 	// natsConn.Close()

// 	sub, err := natspubsub.OpenSubscription(
// 		natsConn,
// 		"com.infratographer.events.*.*",
// 		&natspubsub.SubscriptionOptions{Queue: "permissionsapi"})
// 	if err != nil {
// 		return nil, err
// 	}

// 	logger = logger.Named("worker")

// 	return &Subscription{sub: sub, logger: logger}, nil
// }

func HackySendMsg(ctx context.Context, t string, msg *Message) error {
	natsConn, err := nats.Connect("nats://localhost")
	if err != nil {
		return err
	}
	defer natsConn.Close()

	topic, err := natspubsub.OpenTopic(natsConn, t, nil)
	if err != nil {
		return err
	}

	// nolint:errcheck // TODO: figure out how to handle this error
	defer topic.Shutdown(ctx)

	return SendMsg(topic, msg)
}

func SendMsg(topic *pubsub.Topic, msg *Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return topic.Send(context.Background(), &pubsub.Message{
		Body: msgBytes,
	})
}

func (s *Subscription) Receive(ctx context.Context, st *query.Stores) error {
	ctx, span := tracer.Start(ctx, "HandleMessage")
	defer span.End()

	msg, err := s.sub.Receive(ctx)
	if err != nil {
		// Errors from Receive indicate that Receive will no longer succeed.
		log.Printf("Receiving message: %v", err)
		return fmt.Errorf("failed to receive message: %w", err)
	}
	// Do work based on the message, for example:
	// fmt.Printf("Got message: %q\n", msg.Body)

	var em *Message

	err = json.Unmarshal(msg.Body, &em)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	err = s.ProcessMessage(ctx, st, em)
	if err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	// Messages must always be acknowledged with Ack.
	msg.Ack()

	return nil
}

func (s *Subscription) ProcessMessage(ctx context.Context, db *query.Stores, msg *Message) error {
	switch {
	case strings.HasSuffix(msg.EventType, ".added"):
		resource, err := query.NewResourceFromURN(msg.SubjectURN)
		if err != nil {
			fmt.Println("Error getting resource from URN for subject")
			return err
		}

		resource.Fields = msg.SubjectFields

		actor, err := query.NewResourceFromURN(msg.ActorURN)
		if err != nil {
			fmt.Println("Error getting resource from URN for actor")
			return err
		}

		_, err = query.CreateSpiceDBRelationships(ctx, db.SpiceDB, resource, actor)
		if err != nil {
			fmt.Println("Failed to create resource...oh well")
			fmt.Printf("Error: %+v", err)
		} else {
			// fmt.Println("Resource created...")

			// poor sampling. Don't print every single created message, that is so many messages. Instead aim for like 1 out of every 200
			if rand.Intn(10) == 9 {
				s.logger.Infow("created resource",
					"type", resource.ResourceType.Name,
					"id", resource.Fields["id"],
					"name", resource.Fields["name"],
				)

			}
		}
	default:
		fmt.Printf("I don't care about %s\n", msg.EventType)
	}

	return nil
}
