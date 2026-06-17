package convex

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const Version = "0.1.0"

// UserIdentityAttributes contains identity claims for admin impersonation.
// It intentionally permits custom JSON claims, matching Convex's JS client.
type UserIdentityAttributes map[string]any

// HTTPClient calls Convex queries, mutations, actions, and generic functions
// over Convex's HTTP API. This mirrors convex-js' ConvexHttpClient.
type HTTPClient struct {
	address                string
	httpClient             *http.Client
	clientID               string
	skipDeploymentURLCheck bool
	mutationQueue          chan struct{}

	mu           sync.RWMutex
	auth         string
	adminAuth    string
	authCallback UserTokenFetcher
}

// Client is the primary Convex client facade. It uses HTTP for Query,
// Mutation, and Action, and initializes realtime subscriptions lazily.
type Client struct {
	httpClient    *HTTPClient
	deploymentURL string

	mu               sync.Mutex
	webSocketClient  *WebSocketClient
	webSocketOptions []WebSocketOption
	authToken        string
	adminAuthToken   string
	adminActingAs    []UserIdentityAttributes
	authCallback     UserTokenFetcher

	connectionStateObservers      map[uint64]*connectionStateObserver
	nextConnectionStateObserverID uint64
}

// Option configures a Client.
type Option interface {
	applyHTTPOption(*HTTPClient) error
	applyClientOption(*Client) error
}

type optionFunc struct {
	applyHTTP   func(*HTTPClient) error
	applyClient func(*Client) error
}

func (f optionFunc) applyHTTPOption(client *HTTPClient) error {
	if f.applyHTTP == nil {
		return nil
	}
	return f.applyHTTP(client)
}

func (f optionFunc) applyClientOption(client *Client) error {
	if f.applyClient == nil {
		return nil
	}
	return f.applyClient(client)
}

// MutationOption configures an HTTP mutation call.
type MutationOption func(*mutationOptions)

type mutationOptions struct {
	skipQueue bool
}

// WithSkipMutationQueue makes a mutation execute immediately instead of using
// the client's default serial mutation queue.
func WithSkipMutationQueue() MutationOption {
	return func(o *mutationOptions) {
		o.skipQueue = true
	}
}

// WithHTTPClient uses the provided HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	apply := func(c *HTTPClient) error {
		if httpClient == nil {
			return fmt.Errorf("convex: nil http client")
		}
		c.httpClient = httpClient
		return nil
	}
	return optionFunc{applyHTTP: apply}
}

// WithClientID sets the Convex-Client header value.
func WithClientID(clientID string) Option {
	applyHTTP := func(c *HTTPClient) error {
		if strings.TrimSpace(clientID) == "" {
			return fmt.Errorf("convex: client ID cannot be empty")
		}
		c.clientID = clientID
		return nil
	}
	applyClient := func(c *Client) error {
		c.webSocketOptions = append(c.webSocketOptions, WithWebSocketClientID(clientID))
		return nil
	}
	return optionFunc{applyHTTP: applyHTTP, applyClient: applyClient}
}

// WithAuth sets a bearer auth token for public functions.
func WithAuth(token string) Option {
	applyHTTP := func(c *HTTPClient) error {
		c.auth = token
		return nil
	}
	applyClient := func(c *Client) error {
		c.authToken = token
		c.adminAuthToken = ""
		c.adminActingAs = nil
		return nil
	}
	return optionFunc{applyHTTP: applyHTTP, applyClient: applyClient}
}

// WithAdminAuth sets a Convex admin auth token for internal/system functions.
func WithAdminAuth(token string, actingAs ...UserIdentityAttributes) Option {
	applyHTTP := func(c *HTTPClient) error {
		adminAuth, err := encodeAdminAuth(token, actingAs...)
		if err != nil {
			return err
		}
		c.adminAuth = adminAuth
		return nil
	}
	applyClient := func(c *Client) error {
		if _, err := encodeAdminAuth(token, actingAs...); err != nil {
			return err
		}
		c.adminAuthToken = token
		c.adminActingAs = cloneUserIdentityAttributes(actingAs)
		c.authToken = ""
		return nil
	}
	return optionFunc{applyHTTP: applyHTTP, applyClient: applyClient}
}

// WithSkipDeploymentURLCheck allows URLs such as .convex.site.
func WithSkipDeploymentURLCheck() Option {
	return optionFunc{applyHTTP: func(c *HTTPClient) error {
		c.skipDeploymentURLCheck = true
		return nil
	}}
}

// NewHTTPClient creates a Convex HTTP client for a deployment URL such as
// https://happy-animal-123.convex.cloud.
func NewHTTPClient(deploymentURL string, opts ...Option) (*HTTPClient, error) {
	c := &HTTPClient{
		httpClient:    http.DefaultClient,
		clientID:      "go-" + Version,
		mutationQueue: newReadyMutationQueue(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.applyHTTPOption(c); err != nil {
			return nil, err
		}
	}
	address, err := normalizeDeploymentURL(deploymentURL, c.skipDeploymentURLCheck)
	if err != nil {
		return nil, err
	}
	c.address = address
	return c, nil
}

func newReadyMutationQueue() chan struct{} {
	queue := make(chan struct{}, 1)
	select {
	case queue <- struct{}{}:
	default:
	}
	return queue
}

// NewClient creates the primary Convex client facade.
func NewClient(deploymentURL string, opts ...Option) (*Client, error) {
	httpClient, err := NewHTTPClient(deploymentURL, opts...)
	if err != nil {
		return nil, err
	}
	client := &Client{
		httpClient:    httpClient,
		deploymentURL: deploymentURL,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.applyClientOption(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func normalizeDeploymentURL(deploymentURL string, skipCheck bool) (string, error) {
	if strings.TrimSpace(deploymentURL) == "" {
		return "", fmt.Errorf("convex: deployment URL cannot be empty")
	}
	parsed, err := url.Parse(deploymentURL)
	if err != nil {
		return "", fmt.Errorf("convex: invalid deployment URL %q: %w", deploymentURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("convex: deployment URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("convex: deployment URL must include a host")
	}
	if !skipCheck && strings.HasSuffix(parsed.Host, ".convex.site") {
		return "", fmt.Errorf("convex: deployment URL %q ends with .convex.site, which is for HTTP Actions; use the .convex.cloud deployment URL or WithSkipDeploymentURLCheck", deploymentURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func encodeAdminAuth(token string, actingAs ...UserIdentityAttributes) (string, error) {
	if len(actingAs) == 0 || actingAs[0] == nil {
		return token, nil
	}
	if len(actingAs) > 1 {
		return "", fmt.Errorf("convex: expected at most one acting-as identity")
	}
	identityJSON, err := json.Marshal(actingAs[0])
	if err != nil {
		return "", fmt.Errorf("convex: failed to encode acting-as identity: %w", err)
	}
	return token + ":" + base64.StdEncoding.EncodeToString(identityJSON), nil
}

func cloneUserIdentityAttributes(in []UserIdentityAttributes) []UserIdentityAttributes {
	if len(in) == 0 {
		return nil
	}
	out := make([]UserIdentityAttributes, len(in))
	for i, attrs := range in {
		if attrs == nil {
			continue
		}
		clone := make(UserIdentityAttributes, len(attrs))
		for key, value := range attrs {
			clone[key] = value
		}
		out[i] = clone
	}
	return out
}

func syncIdentityAttributes(actingAs []UserIdentityAttributes) []SyncUserIdentityAttributes {
	if len(actingAs) == 0 {
		return nil
	}
	out := make([]SyncUserIdentityAttributes, 0, len(actingAs))
	for _, attrs := range actingAs {
		if attrs == nil {
			continue
		}
		data, err := json.Marshal(attrs)
		if err != nil {
			continue
		}
		var identity SyncUserIdentityAttributes
		if err := json.Unmarshal(data, &identity); err != nil {
			continue
		}
		out = append(out, identity)
	}
	return out
}

func mutationOptionsFrom(opts ...MutationOption) mutationOptions {
	var options mutationOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}
