package slc

import (
	"context"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func (c *Contract) Connect(ctx context.Context, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig, opts ...nats.Option) (jetstream.JetStream, jetstream.Consumer, error) {
	nc, err := nats.Connect(c.Network.EventURL, opts...)
	if err != nil {
		return nil, nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, nil, err
	}

	s, err := js.CreateStream(ctx, streamCfg)
	if err != nil {
		return nil, nil, err
	}
	consumer, _ := s.CreateOrUpdateConsumer(ctx, consumerCfg)
	return js, consumer, nil
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
