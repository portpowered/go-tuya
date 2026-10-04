package tuya

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	clientFixtureSyntheticAccess   = "synthetic-access"
	clientFixtureSyntheticClientID = "synthetic-client-id"
)

//nolint:cyclop,funlen // This test checks each constructor option and the resulting account-session wiring together.
func TestNewClientAppliesOptionsAndCreatesAccountSessions(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errTestUnexpectedRequest
	})
	httpClient := &http.Client{Transport: transport}

	var mqttFactory MQTTClientFactory = func(*mqtt.ClientOptions) mqtt.Client { return nil }

	rtcSignaling := &recordingRTCSignaling{}

	client, err := newSyntheticClient(
		WithHTTPClient(httpClient),
		WithClientID(clientFixtureSyntheticClientID),
		WithAuthenticationURL("https://auth.example.test/"),
		WithCloudAPIURL("https://cloud.example.test/"),
		WithMQTTClientFactory(mqttFactory),
		WithRTCSignaling(rtcSignaling),
	)
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	firstTokens := Tokens{AccessToken: "synthetic-access-one", RefreshToken: "synthetic-refresh-one", ExpireTime: 1000}
	secondTokens := Tokens{AccessToken: "synthetic-access-two", RefreshToken: "synthetic-refresh-two", ExpireTime: 2000}
	first := client.NewSession(firstTokens)
	second := client.NewSession(secondTokens)

	if first.HTTPClient != httpClient {
		t.Fatal("session did not retain the configured HTTP client")
	}

	if first.HTTPClient.Transport == nil {
		t.Fatal("configured HTTP client has no transport")
	}

	if first.ClientID != clientFixtureSyntheticClientID {
		t.Errorf("ClientID = %q, want configured value", first.ClientID)
	}

	if first.AuthenticationURL != "https://auth.example.test" {
		t.Errorf("AuthenticationURL = %q, want trimmed URL", first.AuthenticationURL)
	}

	if first.CloudAPIURL != "https://cloud.example.test" {
		t.Errorf("CloudAPIURL = %q, want trimmed URL", first.CloudAPIURL)
	}

	if first.rtcSignaling != rtcSignaling {
		t.Fatal("session did not use configured RTC signaling client")
	}

	if first.MessageQueue == second.MessageQueue || first.MessageQueue.State == second.MessageQueue.State {
		t.Fatal("sessions share message queue state")
	}

	if got := first.Tokens(); got != firstTokens {
		t.Errorf("first session tokens = %+v, want %+v", got, firstTokens)
	}

	if got := second.Tokens(); got != secondTokens {
		t.Errorf("second session tokens = %+v, want %+v", got, secondTokens)
	}

	updated := Tokens{AccessToken: "synthetic-access-updated", RefreshToken: "synthetic-refresh-updated", ExpireTime: 3000}
	first.SetTokens(updated)

	if got := first.Tokens(); got != updated {
		t.Errorf("updated tokens = %+v, want %+v", got, updated)
	}

	if got := second.Tokens(); got != secondTokens {
		t.Errorf("updating first session changed second session tokens: %+v", got)
	}
}

func TestNewClientTransportDefaults(t *testing.T) {
	t.Parallel()

	client, err := newSyntheticClient()
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	session := client.NewSession(Tokens{})
	if session.HTTPClient == nil || session.mqttClientFactory == nil || session.rtcSignaling == nil {
		t.Fatal("default network edges were not initialized")
	}

	if session.ClientID != clientFixtureSyntheticClientID {
		t.Errorf("ClientID = %q, want configured %q", session.ClientID, clientFixtureSyntheticClientID)
	}

	if session.AuthenticationURL != LoginURI {
		t.Errorf("AuthenticationURL = %q, want default %q", session.AuthenticationURL, LoginURI)
	}

	if session.CloudAPIURL != regionAPIEndpointUS {
		t.Errorf("CloudAPIURL = %q, want default US endpoint", session.CloudAPIURL)
	}
}

func TestWithRegionSelectsCloudEndpoint(t *testing.T) {
	t.Parallel()

	client, err := newSyntheticClient(WithRegion(TuyaRegionEU))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	if got, want := client.NewSession(Tokens{}).CloudAPIURL, regionAPIEndpointEU; got != want {
		t.Errorf("cloud API URL = %q, want %q", got, want)
	}
}

func TestNewClientRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option Option
	}{
		{name: "nil option"},
		{name: "nil HTTP client", option: WithHTTPClient(nil)},
		{name: "nil HTTP transport", option: WithHTTPTransport(nil)},
		{name: "empty client ID", option: WithClientID(" \t")},
		{name: "relative authentication URL", option: WithAuthenticationURL("/login")},
		{name: "non HTTP cloud URL", option: WithCloudAPIURL("mqtt://cloud.example.test")},
		{name: "unsupported region", option: WithRegion(Region("not-a-region"))},
		{name: "nil MQTT factory", option: WithMQTTClientFactory(nil)},
		{name: "nil RTC signaling", option: WithRTCSignaling(nil)},
		{name: "custom option error", option: func(*clientOptions) error { return errTestSyntheticOption }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := newSyntheticClient(test.option); err == nil {
				t.Fatal("newSyntheticClient() error = nil, want validation error")
			}
		})
	}
}

func TestNewClientRejectsConflictingHTTPOptions(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errTestUnexpectedRequest })
	for _, options := range [][]Option{
		{WithHTTPClient(&http.Client{}), WithHTTPTransport(transport)},
		{WithHTTPTransport(transport), WithHTTPClient(&http.Client{})},
	} {
		if _, err := newSyntheticClient(options...); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("conflicting HTTP options: got %v", err)
		}
	}
}

func TestWithHTTPTransportIsUsedByAuthenticationRequests(t *testing.T) {
	t.Parallel()

	called := false
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		called = true

		if request.URL.Host != "auth.example.test" || request.URL.Path != "/v1.0/m/life/home-assistant/qrcode/tokens" {
			t.Errorf("unexpected authentication request URL: %s", request.URL)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       http.NoBody,
			Request:    request,
		}, nil
	})

	client, err := newSyntheticClient(WithHTTPTransport(transport), WithAuthenticationURL("https://auth.example.test"))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	session := client.NewSession(Tokens{})
	if _, err := session.AuthService.GenerateQrCodeForLogin(context.Background(), LoginRequest{
		Schema: authFixtureSchema, AccessCode: "synthetic-code",
	}); err == nil {
		t.Fatal("expected malformed empty mock response error")
	}

	if !called {
		t.Fatal("configured HTTP RoundTripper was not called")
	}
}

func TestClientDoesNotRefreshTokensDuringRequests(t *testing.T) {
	t.Parallel()

	requests := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++

		return nil, errTestRequestRequiresTokens
	})

	client, err := newSyntheticClient(WithHTTPTransport(transport))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	session := client.NewSession(Tokens{})

	_, err = session.EncryptedClient.Get(context.Background(), "/v1.0/devices", nil, testOperationRequest{})
	if err == nil || !strings.Contains(err.Error(), "refresh token is required") {
		t.Fatalf("request error = %v, want an explicit missing-token error", err)
	}

	if requests != 0 {
		t.Errorf("HTTP requests = %d, want 0 when tokens are missing", requests)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingRTCSignaling struct {
	startRequest StartRTCStreamRequest
	stopRequest  StopRTCStreamRequest
}

func (s *recordingRTCSignaling) Start(_ context.Context, request StartRTCStreamRequest) (RTCSessionInfo, error) {
	s.startRequest = request

	return RTCSessionInfo{SessionID: "synthetic-rtc-session", SDPAnswer: "synthetic-sdp-answer"}, nil
}

func (s *recordingRTCSignaling) Stop(_ context.Context, request StopRTCStreamRequest) error {
	s.stopRequest = request

	return nil
}

func TestRTCSignalingCanBeReplaced(t *testing.T) {
	t.Parallel()

	signaling := &recordingRTCSignaling{}

	client, err := newSyntheticClient(WithRTCSignaling(signaling))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	session := client.NewSession(Tokens{})
	startRequest := StartRTCStreamRequest{DeviceID: "synthetic-camera", SDPOffer: "synthetic-sdp-offer"}

	stream, err := session.StartRTCStream(context.Background(), startRequest)
	if err != nil {
		t.Fatalf("StartRTCStream() error = %v", err)
	}

	if stream.GetSessionID() != "synthetic-rtc-session" || stream.GetSDPAnswer() != "synthetic-sdp-answer" {
		t.Fatalf("stream does not reflect injected RTC session: id=%q answer=%q", stream.GetSessionID(), stream.GetSDPAnswer())
	}

	if signaling.startRequest.DeviceID != startRequest.DeviceID || signaling.startRequest.SDPOffer != startRequest.SDPOffer {
		t.Errorf("injected signaling received request %+v, want %+v", signaling.startRequest, startRequest)
	}

	stopRequest := StopRTCStreamRequest{DeviceID: startRequest.DeviceID, SessionID: stream.GetSessionID()}
	if err := session.StopRTCStream(context.Background(), stopRequest); err != nil {
		t.Fatalf("StopRTCStream() error = %v", err)
	}

	if signaling.stopRequest != stopRequest {
		t.Errorf("injected signaling received stop request %+v, want %+v", signaling.stopRequest, stopRequest)
	}
}

func TestMQTTClientFactoryCanBeReplaced(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		response := `{"success":true,"result":{"url":"tcp://mqtt.example.test:1883","clientId":"` +
			syntheticMQTTClientID + `","username":"` + syntheticMQTTUsername + `","password":"` + syntheticMQTTPassword + `",` +
			`"expireTime":3600,"topic":{"ownerId":{"sub":"cloud/owner/{ownerId}/in/#"},` +
			`"devId":{"sub":"cloud/device/{devId}/in/#"}}}}`
		_, _ = writer.Write([]byte(response))
	}))
	defer server.Close()

	factoryCalls := 0

	var receivedOptions *mqtt.ClientOptions

	client, err := newSyntheticClient(
		WithHTTPClient(server.Client()),
		WithCloudAPIURL(server.URL),
		WithMQTTClientFactory(func(options *mqtt.ClientOptions) mqtt.Client {
			factoryCalls++
			receivedOptions = options

			return &stubMQTTClient{}
		}),
	)
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	session := client.NewSession(Tokens{AccessToken: clientFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})
	if err := session.MessageQueue.State.connectMQTT(context.Background(), session.MessageQueue); err != nil {
		t.Fatalf("connectMQTT() error = %v", err)
	}

	if factoryCalls != 1 {
		t.Errorf("MQTT client factory calls = %d, want 1", factoryCalls)
	}

	if receivedOptions == nil || receivedOptions.ClientID != syntheticMQTTClientID {
		t.Errorf("factory received MQTT options %#v, expected configured client ID", receivedOptions)
	}
}

type stubMQTTClient struct {
	mqtt.Client
}

func (*stubMQTTClient) Connect() mqtt.Token { //nolint:ireturn // Paho requires this interface return for mqtt.Client implementations.
	return stubMQTTToken{}
}

func (*stubMQTTClient) IsConnected() bool {
	return false
}

type stubMQTTToken struct {
	mqtt.Token
}

func (stubMQTTToken) Wait() bool { return true }

func (stubMQTTToken) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)

	return done
}

func (stubMQTTToken) Error() error { return nil }

func TestCloseIsSafeWithoutRunningMessageQueue(t *testing.T) {
	t.Parallel()

	client, err := newSyntheticClient()
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	if err := client.NewSession(Tokens{}).Close(context.Background()); err != nil {
		t.Errorf("Close() on a stopped session = %v, want nil", err)
	}
}

func TestValidateEndpoint(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{"", "/relative", "file:///tmp/endpoint", "https:///missing-host", "http://"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			err := validateEndpoint("test endpoint", endpoint)
			if err == nil {
				t.Errorf("validateEndpoint(%q) error = nil, want validation error", endpoint)
			}
		})
	}

	err := validateEndpoint("test endpoint", "http://example.test/path")
	if err != nil {
		t.Errorf("validateEndpoint() for absolute HTTP URL = %v", err)
	}
}

func TestAuthClientHTTPTransportCanBeReusedAcrossSessions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"success":true,"result":{"qrcode":"synthetic-code"}}`))
	}))
	defer server.Close()

	client, err := newSyntheticClient(WithHTTPClient(server.Client()), WithAuthenticationURL(server.URL))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	request := LoginRequest{Schema: authFixtureSchema, AccessCode: "synthetic-access-code"}
	for range 2 {
		_, err := client.NewSession(Tokens{}).AuthService.GenerateQrCodeForLogin(
			context.Background(),
			request,
		)
		if err != nil {
			t.Fatalf("GenerateQrCodeForLogin() error = %v", err)
		}
	}
}

func newSyntheticClient(options ...Option) (*Client, error) {
	fixtureOptions := make([]Option, 0, 1+len(options))
	fixtureOptions = append(fixtureOptions, WithClientID(clientFixtureSyntheticClientID))
	fixtureOptions = append(fixtureOptions, options...)

	return NewClient(fixtureOptions...)
}

func TestNewClientRequiresApplicationIdentity(t *testing.T) {
	t.Parallel()

	_, err := NewClient()
	if err == nil || !strings.Contains(err.Error(), "client ID") {
		t.Fatalf("NewClient() error = %v, want missing application identity", err)
	}
}
