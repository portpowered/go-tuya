package tuya

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var errHTTPClientCookieJar = errors.New("HTTP client CookieJar must be nil; configure account cookies on each Session.HTTPClient")

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
// credentials, cookies, and live connections belong to Session values created
// from it.
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
//
// Deprecated: use Tokens.
type AuthInformation = Tokens

// NewClient creates a reusable Tuya client from functional options.
// WithClientID must supply the caller's Tuya application identity.
func NewClient(options ...Option) (*Client, error) {
	configured := clientOptions{
		httpClient:        new(http.Client),
		httpClientSet:     false,
		httpTransportSet:  false,
		clientID:          "",
		authenticationURL: LoginURI,
		cloudAPIURL:       regionAPIEndpointUS,
		mqttClientFactory: newMQTTClient,
		rtcSignaling:      nil,
	}

	for index, option := range options {
		if option == nil {
			return nil, clientError(ErrorInvalidOperation, fmt.Errorf("%w %d is nil", errNilClientOption, index))
		}

		err := option(&configured)
		if err != nil {
			return nil, clientError(ErrorInvalidOperation, fmt.Errorf("client option %d: %w", index, err))
		}
	}

	if configured.httpClient == nil {
		return nil, clientError(ErrorInvalidOperation, errHTTPClientRequired)
	}

	if configured.httpClient.Jar != nil {
		return nil, clientError(ErrorInvalidOperation, errHTTPClientCookieJar)
	}

	if strings.TrimSpace(configured.clientID) == "" {
		return nil, clientError(ErrorInvalidOperation, errClientIDRequired)
	}

	err := validateEndpoint("authentication URL", configured.authenticationURL)
	if err != nil {
		return nil, clientError(ErrorInvalidOperation, err)
	}

	err = validateEndpoint("cloud API URL", configured.cloudAPIURL)
	if err != nil {
		return nil, clientError(ErrorInvalidOperation, err)
	}

	if configured.mqttClientFactory == nil {
		return nil, clientError(ErrorInvalidOperation, errMQTTFactoryRequired)
	}

	configured.httpClient = cloneHTTPClient(configured.httpClient)

	return &Client{options: configured}, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return nil
	}

	clone := *client

	return &clone
}

// WithHTTPClient uses a caller-configured HTTP client for all HTTP operations,
// including Tuya's HTTP-based RTC signaling requests. Its CookieJar must be
// nil. NewClient rejects a configured jar and snapshots the client value;
// each Session receives its own client value. To use account cookies, set a
// separate Jar on that session's HTTPClient.
func WithHTTPClient(client *http.Client) Option {
	return func(options *clientOptions) error {
		if client == nil {
			return errHTTPClientNil
		}

		if options.httpClientSet || options.httpTransportSet {
			return errHTTPOptionsConflict
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
			return errHTTPTransportNil
		}

		if options.httpClientSet || options.httpTransportSet {
			return errHTTPOptionsConflict
		}

		httpClient := new(http.Client)
		httpClient.Transport = transport
		options.httpClient = httpClient
		options.httpTransportSet = true

		return nil
	}
}

// WithClientID supplies the caller's Tuya application client ID.
func WithClientID(clientID string) Option {
	return func(options *clientOptions) error {
		if strings.TrimSpace(clientID) == "" {
			return errClientIDRequired
		}

		options.clientID = clientID

		return nil
	}
}

// WithAuthenticationURL overrides the QR login and token validation endpoint.
func WithAuthenticationURL(endpoint string) Option {
	return func(options *clientOptions) error {
		err := validateEndpoint("authentication URL", endpoint)
		if err != nil {
			return err
		}

		options.authenticationURL = strings.TrimRight(endpoint, "/")

		return nil
	}
}

// WithCloudAPIURL overrides the Tuya cloud endpoint.
func WithCloudAPIURL(endpoint string) Option {
	return func(options *clientOptions) error {
		err := validateEndpoint("cloud API URL", endpoint)
		if err != nil {
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
			return errMQTTFactoryNil
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
			return errRTCSignalingNil
		}

		options.rtcSignaling = signaling

		return nil
	}
}

func validateEndpoint(name, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return fmt.Errorf("%s %w without credentials, query, or fragment", name, errAbsoluteHTTPURL)
	}

	return nil
}

// NewSession creates an account-scoped session. Its tokens and connection
// lifecycle are isolated from the reusable Client and other sessions.
func (c *Client) NewSession(tokens Tokens) *Session {
	session := new(Session)
	session.HTTPClient = cloneHTTPClient(c.options.httpClient)
	session.CloudAPIURL = c.options.cloudAPIURL
	session.ClientID = c.options.clientID
	session.AuthenticationURL = c.options.authenticationURL
	session.mqttClientFactory = c.options.mqttClientFactory
	session.rtcSignaling = c.options.rtcSignaling
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

	_, err := s.MessageQueue.Stop(ctx, MessageQueueStopRequest{Request: Request{AuthorizationContext: nil, DoNotRefreshToken: false}})

	return err
}

// NewMessageQueue creates a message queue with state owned by the supplied session.
func NewMessageQueue(session *Session) *SharingMessageQueueImpl {
	state := new(mqttState)
	state.messageListeners = make(map[string][]MessageListener)
	state.deviceListeners = make(map[string][]DeviceListener)

	return &SharingMessageQueueImpl{
		Client: session,
		State:  state,
	}
}

// Unload ends the session. Use Close for the idiomatic session lifecycle API.
func (s *Session) Unload(ctx context.Context, _ UnloadRequest) (UnloadResponse, error) {
	err := s.Close(ctx)
	if err != nil {
		return UnloadResponse{}, err
	}

	return UnloadResponse{}, nil
}
