package tuya

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Option configures a reusable Client.
type Option func(*clientOptions) error

type clientOptions struct {
	httpClient        *http.Client
	httpClientSet     bool
	httpTransportSet  bool
	clientID          string
	authenticationURL string
	cloudAPIURL       string
	mqttClientFactory MQTTClientFactory
	rtcSignaling      RTCSignaling
}

// Client contains reusable endpoint and transport configuration. Account
// credentials and live connections belong to Session values created from it.
type Client struct {
	options clientOptions
}

// Tokens contains an account's current authentication credentials. Callers
// own persistence and explicitly pass rotated credentials back to a session.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	// ExpireTime is milliseconds since the Unix epoch.
	ExpireTime int64
}

// AuthInformation is retained as a source-compatible name for Tokens.
// Deprecated: use Tokens.
type AuthInformation = Tokens

// NewClient creates a reusable Tuya client from functional options.
func NewClient(options ...Option) (*Client, error) {
	configured := clientOptions{
		httpClient:        &http.Client{},
		clientID:          clientID,
		authenticationURL: LoginURI,
		cloudAPIURL:       regionEndpoints[TuyaRegionUS],
		mqttClientFactory: newMQTTClient,
	}
	for index, option := range options {
		if option == nil {
			return nil, clientError(ErrorInvalidOperation, fmt.Errorf("client option %d is nil", index))
		}
		if err := option(&configured); err != nil {
			return nil, clientError(ErrorInvalidOperation, fmt.Errorf("client option %d: %w", index, err))
		}
	}
	if configured.httpClient == nil {
		return nil, clientError(ErrorInvalidOperation, fmt.Errorf("HTTP client is required"))
	}
	if strings.TrimSpace(configured.clientID) == "" {
		return nil, clientError(ErrorInvalidOperation, fmt.Errorf("client ID must not be empty"))
	}
	if err := validateEndpoint("authentication URL", configured.authenticationURL); err != nil {
		return nil, clientError(ErrorInvalidOperation, err)
	}
	if err := validateEndpoint("cloud API URL", configured.cloudAPIURL); err != nil {
		return nil, clientError(ErrorInvalidOperation, err)
	}
	if configured.mqttClientFactory == nil {
		return nil, clientError(ErrorInvalidOperation, fmt.Errorf("MQTT client factory is required"))
	}
	return &Client{options: configured}, nil
}

// WithHTTPClient uses a caller-configured HTTP client for all HTTP operations,
// including Tuya's HTTP-based RTC signaling requests.
func WithHTTPClient(client *http.Client) Option {
	return func(options *clientOptions) error {
		if client == nil {
			return fmt.Errorf("HTTP client must not be nil")
		}
		if options.httpClientSet || options.httpTransportSet {
			return fmt.Errorf("HTTP client and transport options are mutually exclusive")
		}
		options.httpClient = client
		options.httpClientSet = true
		return nil
	}
}

// WithHTTPTransport uses a caller-provided RoundTripper for HTTP operations.
// Use an http.Transport with ForceAttemptHTTP2 set to enable HTTP/2.
func WithHTTPTransport(transport http.RoundTripper) Option {
	return func(options *clientOptions) error {
		if transport == nil {
			return fmt.Errorf("HTTP transport must not be nil")
		}
		if options.httpClientSet || options.httpTransportSet {
			return fmt.Errorf("HTTP client and transport options are mutually exclusive")
		}
		options.httpClient = &http.Client{Transport: transport}
		options.httpTransportSet = true
		return nil
	}
}

// WithClientID overrides the provider client ID.
func WithClientID(id string) Option {
	return func(options *clientOptions) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("client ID must not be empty")
		}
		options.clientID = id
		return nil
	}
}

// WithAuthenticationURL overrides the QR login and token validation endpoint.
func WithAuthenticationURL(endpoint string) Option {
	return func(options *clientOptions) error {
		if err := validateEndpoint("authentication URL", endpoint); err != nil {
			return err
		}
		options.authenticationURL = strings.TrimRight(endpoint, "/")
		return nil
	}
}

// WithCloudAPIURL overrides the Tuya cloud endpoint.
func WithCloudAPIURL(endpoint string) Option {
	return func(options *clientOptions) error {
		if err := validateEndpoint("cloud API URL", endpoint); err != nil {
			return err
		}
		options.cloudAPIURL = strings.TrimRight(endpoint, "/")
		return nil
	}
}

// WithRegion selects a built-in Tuya cloud endpoint.
func WithRegion(region Region) Option {
	return func(options *clientOptions) error {
		endpoint, err := GetRegionEndpoint(region)
		if err != nil {
			return err
		}
		options.cloudAPIURL = endpoint
		return nil
	}
}

// WithMQTTClientFactory injects creation of the message-queue client.
func WithMQTTClientFactory(factory MQTTClientFactory) Option {
	return func(options *clientOptions) error {
		if factory == nil {
			return fmt.Errorf("MQTT client factory must not be nil")
		}
		options.mqttClientFactory = factory
		return nil
	}
}

// WithRTCSignaling replaces the default encrypted HTTP signaling path for
// camera sessions. Implementations may use another network edge, such as a
// WebSocket. The implementation must be safe for concurrent use by sessions
// created from the same Client.
func WithRTCSignaling(signaling RTCSignaling) Option {
	return func(options *clientOptions) error {
		if signaling == nil {
			return fmt.Errorf("RTC signaling client must not be nil")
		}
		options.rtcSignaling = signaling
		return nil
	}
}

func validateEndpoint(name, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL", name)
	}
	return nil
}

// NewSession creates an account-scoped session. Its tokens and connection
// lifecycle are isolated from the reusable Client and other sessions.
func (c *Client) NewSession(tokens Tokens) *Session {
	session := &Session{
		HTTPClient:        c.options.httpClient,
		CloudAPIURL:       c.options.cloudAPIURL,
		ClientID:          c.options.clientID,
		AuthenticationURL: c.options.authenticationURL,
		mqttClientFactory: c.options.mqttClientFactory,
		rtcSignaling:      c.options.rtcSignaling,
	}
	session.initialize(tokens)
	return session
}

func (s *Session) initialize(tokens Tokens) {
	s.EncryptedClient = &EncryptedClient{Client: s}
	if s.rtcSignaling == nil {
		s.rtcSignaling = encryptedRTCSignaling{client: s.EncryptedClient}
	}
	s.DevicesService = &DevicesService{client: s}
	s.AuthService = &AuthService{client: s}
	s.HomeService = &HomeService{client: s}
	s.UserService = &UserService{client: s}
	s.SceneService = &SceneService{client: s}
	s.MessageQueue = NewMessageQueue(s)
	s.SetTokens(tokens)
}

// Tokens returns a snapshot of the credentials currently assigned to this
// session. Token refresh operations do not update this snapshot automatically.
func (s *Session) Tokens() Tokens {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.tokens
}

// SetTokens explicitly replaces the credentials used by subsequent requests.
func (s *Session) SetTokens(tokens Tokens) {
	s.tokenMu.Lock()
	s.tokens = tokens
	s.tokenMu.Unlock()
}

// Close stops the stateful message queue owned by this session.
func (s *Session) Close(ctx context.Context) error {
	if s.MessageQueue == nil {
		return nil
	}
	_, err := s.MessageQueue.Stop(ctx, MessageQueueStopRequest{})
	return err
}

// NewMessageQueue creates a message queue with state owned by the supplied session.
func NewMessageQueue(session *Session) *SharingMessageQueueImpl {
	return &SharingMessageQueueImpl{
		Client: session,
		State: &mqttState{
			messageListeners: make(map[string][]MessageListener),
			deviceListeners:  make(map[string][]DeviceListener),
		},
	}
}

// Unload ends the session. Use Close for the idiomatic session lifecycle API.
func (s *Session) Unload(ctx context.Context, _ UnloadRequest) (UnloadResponse, error) {
	if err := s.Close(ctx); err != nil {
		return UnloadResponse{}, err
	}
	return UnloadResponse{}, nil
}
