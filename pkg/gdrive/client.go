package gdrive

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jflowers/get-out/pkg/secrets"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Client provides access to Google Drive and Docs APIs.
type Client struct {
	Drive *drive.Service
	Docs  *docs.Service
}

// NewClient creates a new Google Drive/Docs client from an authenticated HTTP client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	docsService, err := docs.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docs service: %w", err)
	}

	return &Client{
		Drive: driveService,
		Docs:  docsService,
	}, nil
}

// NewClientFromConfig creates a client by authenticating with the given config.
func NewClientFromConfig(ctx context.Context, cfg *Config) (*Client, error) {
	httpClient, err := Authenticate(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return NewClient(ctx, httpClient)
}

// NewClientFromStore creates a client using a SecretStore for credential and
// token I/O. This is the preferred function when the SecretStore is available.
func NewClientFromStore(ctx context.Context, cfg *Config, store secrets.SecretStore) (*Client, error) {
	httpClient, err := AuthenticateWithStore(ctx, cfg, store)
	if err != nil {
		return nil, err
	}

	return NewClient(ctx, httpClient)
}
