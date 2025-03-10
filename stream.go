package slc

import (
	"context"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func (c *Contract) Connect(ctx context.Context, opts ...ConnectionOption) (jetstream.JetStream, jetstream.Consumer, error) {
	options := &ConnectionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	nc, err := nats.Connect(c.Network.EventURL)
	if err != nil {
		return nil, nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, nil, err
	}

	if options.JetStream.Name != "" {
		s, err := js.CreateStream(ctx, options.JetStream)
		if err != nil {
			return nil, nil, err
		}
		consumer, _ := s.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			// TODO: Trim <32 characters: https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/naming
			Durable:   formatConsumerName(c.Name, c.ID),
			AckPolicy: jetstream.AckExplicitPolicy,
		})
		return js, consumer, nil
	} else {
		s, err := js.CreateStream(ctx, jetstream.StreamConfig{
			// TODO: Trim <32 characters: https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/naming
			Name: "default",
		})
		if err != nil {
			return nil, nil, err
		}
		consumer, _ := s.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			// TODO: Trim <32 characters: https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/naming
			Durable:   formatConsumerName(c.Name, c.ID),
			AckPolicy: jetstream.AckExplicitPolicy,
		})
		return js, consumer, nil
	}
}

// formatConsumerName formats the name of the Contract to remove any special characters.
func formatConsumerName(name, id string) string {
	name = strings.ToUpper(name)

	// TODO: Add additional formatting rules.

	if id != "" {
		name = name + "-" + id
	}
	return strings.ReplaceAll(name, " ", "-")
}
