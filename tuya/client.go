package tuya

import (
	"context"
	"net/http"
)

// ClientConfig contains configuration information that can be specified for creating a new client.
type ClientConfig struct {
	HTTPClient        *http.Client
	ClientID          *string
	AuthenticationURL *string
	CloudAPIURL       *string
	AuthInformation   *AuthInformation
}

// AuthInformation contains authentication credentials and tokens
type AuthInformation struct {
	AccessToken  string
	RefreshToken string
	// Time of expiry in milliseconds since epoch format.
	ExpireTime int64
}

// NewClient creates and returns a new Tuya client instance.
func NewClient(config *ClientConfig) *ClientImpl {
	if config == nil {
		config = &ClientConfig{
			HTTPClient: &http.Client{},
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}
	c := &ClientImpl{
		HTTPClient:        config.HTTPClient,
		CloudAPIURL:       regionEndpoints[TuyaRegionUS],
		AuthenticationURL: LoginURI,
		ClientID:          clientID,
	}

	if config.ClientID != nil {
		c.ClientID = *config.ClientID
	}
	if config.AuthenticationURL != nil {
		c.AuthenticationURL = *config.AuthenticationURL
	}
	if config.CloudAPIURL != nil {
		c.CloudAPIURL = *config.CloudAPIURL
	}
	c.initialize(*config)
	return c
}

func (c *ClientImpl) initialize(config ClientConfig) {
	c.EncryptedClient = &EncryptedClient{Client: c}
	c.DevicesService = &DevicesService{client: c}
	c.AuthService = &AuthService{client: c}
	c.HomeService = &HomeService{client: c}
	c.UserService = &UserService{client: c}
	c.SceneService = &SceneService{client: c}
	c.TokenProvider = &TokenProviderImpl{Client: c}
	c.MessageQueue = NewMessageQueue(c)
	if config.AuthInformation != nil {
		c.TokenProvider.SetToken(
			config.AuthInformation.AccessToken,
			config.AuthInformation.RefreshToken,
			config.AuthInformation.ExpireTime,
		)
	}
}

// NewMessageQueue creates and returns a new Tuya SharingMessageQueue instance.
func NewMessageQueue(client *ClientImpl) *SharingMessageQueueImpl {
	return &SharingMessageQueueImpl{
		Client: client,
		State: &mqttState{
			messageListeners: make(map[string][]MessageListener),
			deviceListeners:  make(map[string][]DeviceListener),
		},
	}
}

// Manager API implementations

// Unload is a placeholder for client unloading/cleanup, to be done when the client terminates.
func (c *ClientImpl) Unload(_ context.Context, _ UnloadRequest) (UnloadResponse, error) {
	// TODO: implement client unloading/cleanup
	return UnloadResponse{}, nil
}
