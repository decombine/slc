package slc

import (
	"context"
	"errors"
	"net/http"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/zitadel/oidc/v3/pkg/client/profile"
	"golang.org/x/oauth2"
)

// ConnectionOptions holds options for configuring Contract Network connectivity.
type ConnectionOptions struct {
	JWT       []byte
	Secret    string
	JetStream jetstream.StreamConfig
}

type ConnectionOption func(*ConnectionOptions)

// WithToken is a ConnectionOption that changes the default behavior of the Contract Network to use a JSON Web Token (JWT).
func WithToken(token string) ConnectionOption {
	return func(opts *ConnectionOptions) {
		opts.JWT = []byte(token)
	}
}

func WithSecret(secret string) ConnectionOption {
	return func(opts *ConnectionOptions) {
		opts.Secret = secret
	}
}

// WithJetStream is a ConnectionOption to provide a JetStream configuration for the Contract Network.
func WithJetStream(config jetstream.StreamConfig) ConnectionOption {
	return func(opts *ConnectionOptions) {
		opts.JetStream = config
	}
}

// NewClient creates a new Client for a Contract Network.
func NewClient(ctx context.Context, network Network, opts ...ConnectionOption) (*http.Client, error) {
	options := &ConnectionOptions{}
	for _, opt := range opts {
		opt(options)
	}

	scopes := []string{"openid", "profile", `urn:zitadel:iam:org:project:id:` + network.ClientID + `:aud`}

	if options.JWT != nil {
		ts, err := profile.NewJWTProfileTokenSourceFromKeyFileData(ctx, network.Issuer, options.JWT, scopes)
		if err != nil {
			return nil, err
		}
		client := oauth2.NewClient(ctx, ts)
		return client, nil
	}

	return nil, errors.New("no connection options provided")
}
