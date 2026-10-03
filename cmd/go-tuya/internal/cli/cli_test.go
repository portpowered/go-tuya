package cli_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/portpowered/go-tuya/cmd/go-tuya/internal/cli"
)

const (
	fixtureClientID       = "synthetic-client-id"
	fixtureAccess         = "synthetic-access-token"
	fixtureRefresh        = "synthetic-refresh-token"
	fixtureCloudURL       = "https://api.example.invalid"
	fixtureAuthURL        = "https://login.example.invalid"
	fixtureDeviceID       = "synthetic-device-01"
	fixtureOwnerTopic     = "cloud/owner/in/channel"
	fixtureRotatedAccess  = "synthetic-rotated-access"
	fixtureRotatedRefresh = "synthetic-rotated-refresh"
)

type replayRequest struct {
	Method  string              `json:"method"`
	Origin  string              `json:"origin"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type replayResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    json.RawMessage     `json:"body"`
}

type replayPair struct {
	Request  replayRequest  `json:"request"`
	Response replayResponse `json:"response"`
}

type replayFixtures struct {
	Provenance    string       `json:"provenance"`
	HomesList     replayPair   `json:"homes_list"`
	Unauthorized  replayPair   `json:"unauthorized"`
	DeviceStatus  replayPair   `json:"device_status"`
	DeviceCommand replayPair   `json:"device_command"`
	TokenRefresh  replayPair   `json:"token_refresh"`
	QRLogin       []replayPair `json:"qr_login"`
	Events        []replayPair `json:"events"`
}

type pairedTransport struct {
	pairs []replayPair
	next  int
}

var (
	uuidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	signaturePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (transport *pairedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.next >= len(transport.pairs) {
		return nil, fmt.Errorf("unexpected HTTP exchange %s %s", request.Method, request.URL)
	}

	pair := transport.pairs[transport.next]

	err := matchRequest(request, pair.Request)
	if err != nil {
		return nil, fmt.Errorf("paired exchange %d: %w", transport.next, err)
	}

	transport.next++

	return &http.Response{
		StatusCode: pair.Response.Status,
		Header:     http.Header(pair.Response.Headers),
		Body:       io.NopCloser(bytes.NewReader(pair.Response.Body)),
		Request:    request,
	}, nil
}

func (transport *pairedTransport) verifyConsumed() error {
	if transport.next != len(transport.pairs) {
		return fmt.Errorf("consumed %d of %d paired exchanges", transport.next, len(transport.pairs))
	}

	return nil
}

func matchRequest(request *http.Request, expected replayRequest) error {
	origin := request.URL.Scheme + "://" + request.URL.Host
	if request.Method != expected.Method || origin != expected.Origin || request.URL.EscapedPath() != expected.Path {
		return fmt.Errorf(
			"method/origin/path = %s %s%s, want %s %s%s",
			request.Method,
			origin,
			request.URL.EscapedPath(),
			expected.Method,
			expected.Origin,
			expected.Path,
		)
	}

	err := matchValues(request.URL.Query(), expected.Query)
	if err != nil {
		return err
	}

	for name, values := range expected.Headers {
		actual := request.Header.Values(name)
		if len(actual) != len(values) {
			return fmt.Errorf("header %s has %d values, want %d", name, len(actual), len(values))
		}

		for i, value := range values {
			err := matchValue(actual[i], value)
			if err != nil {
				return fmt.Errorf("header %s: %w", name, err)
			}
		}
	}

	var body []byte

	if request.Body != nil {
		readBody, err := io.ReadAll(request.Body)
		if err != nil {
			return fmt.Errorf("read request body: %w", err)
		}

		body = readBody
		request.Body = io.NopCloser(bytes.NewReader(body))
	}

	if expected.Body == "<encrypted-json>" {
		var envelope struct {
			Encdata string `json:"encdata"`
		}

		if json.Unmarshal(body, &envelope) != nil {
			return errors.New("request body is not an encrypted JSON envelope")
		}

		decoded, decodeErr := base64.StdEncoding.DecodeString(envelope.Encdata)
		if decodeErr != nil || len(decoded) <= 12 {
			return errors.New("request body has invalid encrypted data")
		}
	} else if string(body) != expected.Body {
		return errors.New("request body differs from paired fixture")
	}

	return nil
}

func matchValues(actual, expected map[string][]string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("query has %d keys, want %d", len(actual), len(expected))
	}

	for name, values := range expected {
		got := actual[name]
		if len(got) != len(values) {
			return fmt.Errorf("query %s has %d values, want %d", name, len(got), len(values))
		}

		for i, value := range values {
			err := matchValue(got[i], value)
			if err != nil {
				return fmt.Errorf("query %s: %w", name, err)
			}
		}
	}

	return nil
}

func matchValue(actual, expected string) error {
	switch expected {
	case "<uuid>":
		if !uuidPattern.MatchString(actual) {
			return errors.New("value is not a UUID")
		}
	case "<tuya-signature>":
		if !signaturePattern.MatchString(actual) {
			return errors.New("value is not a Tuya signature")
		}
	case "<unix-millis>":
		stamp, err := strconv.ParseInt(actual, 10, 64)
		if err != nil || time.Since(time.UnixMilli(stamp)) > 5*time.Minute || time.Until(time.UnixMilli(stamp)) > 5*time.Minute {
			return errors.New("value is not a recent Unix timestamp")
		}
	case "<encrypted>":
		decoded, err := base64.StdEncoding.DecodeString(actual)
		if err != nil || len(decoded) <= 12 {
			return errors.New("value is not valid encrypted data")
		}
	default:
		if actual != expected {
			return fmt.Errorf("value = %q, want %q", actual, expected)
		}
	}

	return nil
}

func loadFixtures(t *testing.T) replayFixtures {
	t.Helper()

	path := filepath.Join("testdata", "cli-replay.synthetic.json")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var fixtures replayFixtures

	err = json.Unmarshal(data, &fixtures)
	if err != nil {
		t.Fatal(err)
	}

	if fixtures.Provenance == "" {
		t.Fatal("synthetic fixture provenance is missing")
	}

	return fixtures
}

func writeTokenFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.json")

	data, err := json.Marshal(struct { //nolint:gosec // Test data uses synthetic credential strings only.
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpireTime   int64  `json:"expire_time"`
	}{
		AccessToken:  fixtureAccess,
		RefreshToken: fixtureRefresh,
		ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}

	data = append(data, '\n')

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func commandArgs(tokenPath string, command ...string) []string {
	args := make([]string, 0, 7+len(command))
	args = append(args,
		"--token-file", tokenPath,
		"--client-id", fixtureClientID,
		"--cloud-api-url", fixtureCloudURL,
		"--json",
	)

	return append(args, command...)
}

func runCommand(args []string, transport http.RoundTripper, dependencies cli.Dependencies, input io.Reader) (int, string, string) {
	dependencies.HTTPTransport = transport

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	code := cli.Run(context.Background(), args, input, &stdout, &stderr, dependencies)

	return code, stdout.String(), stderr.String()
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	code := cli.Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr, cli.Dependencies{})
	if code != 0 {
		t.Fatalf("Run(--help) exit code = %d, want 0; stderr = %s", code, stderr.String())
	}

	for _, command := range []string{"auth qr", "auth refresh", "device command", "events watch"} {
		if !strings.Contains(stdout.String(), command) {
			t.Errorf("help output does not mention %q", command)
		}
	}
}

func TestAuthQRPollPairedFlow(t *testing.T) {
	t.Parallel()
	fixtures := loadFixtures(t)
	transport := &pairedTransport{pairs: fixtures.QRLogin}
	configDir := t.TempDir()
	tokenPath := filepath.Join(configDir, "tokens.json")
	pendingPath := filepath.Join(configDir, "pending.json")
	accessCodePath := filepath.Join(configDir, "access-code.txt")

	err := os.WriteFile(accessCodePath, []byte("synthetic-user-code\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	var renderedCode string

	dependencies := cli.Dependencies{RenderQRCode: func(_ io.Writer, value string) error {
		renderedCode = value

		return nil
	}}
	common := []string{"--token-file", tokenPath, "--pending-file", pendingPath, "--auth-url", fixtureAuthURL, "--client-id", fixtureClientID, "--json"}
	qrArgs := append(append([]string{}, common...), "auth", "qr", "--access-code-file", accessCodePath)

	code, qrOutput, qrErrors := runCommand(qrArgs, transport, dependencies, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("auth qr exit code = %d, stderr = %s", code, qrErrors)
	}

	if renderedCode != "tuyaSmart--qrLogin?token=synthetic-qr-code-001" {
		t.Fatalf("rendered QR data = %q", renderedCode)
	}

	if strings.Contains(qrOutput+qrErrors, "synthetic-user-code") || strings.Contains(qrOutput+qrErrors, "synthetic-qr-code-001") {
		t.Fatal("auth qr printed the access code or login token")
	}

	if _, err := os.Stat(pendingPath); err != nil {
		t.Fatalf("pending login file was not written: %v", err)
	}

	pollArgs := append(append([]string{}, common...), "auth", "poll")

	code, pollOutput, pollErrors := runCommand(pollArgs, transport, dependencies, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("auth poll exit code = %d, stderr = %s", code, pollErrors)
	}

	if strings.Contains(pollOutput+pollErrors, fixtureAccess) || strings.Contains(pollOutput+pollErrors, fixtureRefresh) {
		t.Fatal("auth poll printed token values")
	}

	if strings.Contains(pollOutput, "authenticated") == false {
		t.Fatal("auth poll did not return a machine-readable success result")
	}

	var saved struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		CloudAPIURL  string `json:"cloud_api_url"`
	}

	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(tokenData, &saved)
	if err != nil {
		t.Fatal(err)
	}

	if saved.AccessToken != fixtureAccess || saved.RefreshToken != fixtureRefresh || saved.CloudAPIURL != fixtureCloudURL {
		t.Fatalf("saved token document = %+v", saved)
	}

	_, err = os.Stat(pendingPath)
	if !os.IsNotExist(err) {
		t.Fatalf("pending login still exists, stat error = %v", err)
	}

	err = transport.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadAndExplicitCommandPairedRequests(t *testing.T) {
	t.Parallel()
	fixtures := loadFixtures(t)
	transport := &pairedTransport{pairs: []replayPair{fixtures.HomesList, fixtures.DeviceStatus, fixtures.Events[1], fixtures.DeviceCommand}}
	tokenPath := writeTokenFile(t)

	code, homesOutput, stderr := runCommand(commandArgs(tokenPath, "homes", "list"), transport, cli.Dependencies{}, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("homes list exit code = %d, stderr = %s", code, stderr)
	}

	if !strings.Contains(homesOutput, "Synthetic Test Home") || !strings.Contains(homesOutput, "synthetic-home-01") {
		t.Fatalf("homes list output = %s", homesOutput)
	}

	code, statusOutput, stderr := runCommand(commandArgs(tokenPath, "devices", "status", fixtureDeviceID), transport, cli.Dependencies{}, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("devices status exit code = %d, stderr = %s", code, stderr)
	}

	var status struct {
		DeviceID string `json:"device_id"`
		Online   bool   `json:"online"`
		Status   []struct {
			Code  string `json:"code"`
			Value any    `json:"value"`
		} `json:"status"`
	}

	err := json.Unmarshal([]byte(statusOutput), &status)
	if err != nil {
		t.Fatal(err)
	}

	if status.DeviceID != fixtureDeviceID || !status.Online || len(status.Status) != 2 || status.Status[0].Code != "switch_1" || status.Status[0].Value != true {
		t.Fatalf("device status = %+v", status)
	}

	if strings.Contains(statusOutput, "local_key") {
		t.Fatal("device status output exposed fields outside its status projection")
	}

	listArgs := commandArgs(tokenPath, "devices", "list", "--home", "synthetic-home-01")

	code, devicesOutput, stderr := runCommand(listArgs, transport, cli.Dependencies{}, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("devices list exit code = %d, stderr = %s", code, stderr)
	}

	if !strings.Contains(devicesOutput, fixtureDeviceID) || strings.Contains(devicesOutput, "local_key") {
		t.Fatalf("devices list output was incomplete or included private fields: %s", devicesOutput)
	}

	commandInput := strings.NewReader("true\n")

	deviceCommandArgs := commandArgs(tokenPath, "device", "command", "--value-stdin", "device-1", "switch")

	code, commandOutput, stderr := runCommand(deviceCommandArgs, transport, cli.Dependencies{}, commandInput)
	if code != 0 {
		t.Fatalf("device command exit code = %d, stderr = %s", code, stderr)
	}

	if !strings.Contains(commandOutput, `"sent": true`) || strings.Contains(commandOutput, fixtureAccess) {
		t.Fatalf("device command output = %s", commandOutput)
	}

	err = transport.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticationFailureIsSafeAndNonzero(t *testing.T) {
	t.Parallel()
	fixtures := loadFixtures(t)
	transport := &pairedTransport{pairs: []replayPair{fixtures.Unauthorized}}
	tokenPath := writeTokenFile(t)

	code, stdout, stderr := runCommand(commandArgs(tokenPath, "homes", "list"), transport, cli.Dependencies{}, strings.NewReader(""))
	if code == 0 {
		t.Fatal("unauthorized request returned success")
	}

	if !strings.Contains(stdout+stderr, `"kind": "unauthorized"`) || !strings.Contains(stdout+stderr, "list homes failed") {
		t.Fatalf("authentication error output = %s%s", stdout, stderr)
	}

	for _, secret := range []string{fixtureAccess, fixtureRefresh, "synthetic token rejected"} {
		if strings.Contains(stdout+stderr, secret) {
			t.Errorf("authentication error exposed %q", secret)
		}
	}

	err := transport.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRefreshAndExportAreExplicitAndPrivate(t *testing.T) {
	t.Parallel()
	fixtures := loadFixtures(t)
	transport := &pairedTransport{pairs: []replayPair{fixtures.TokenRefresh}}
	tokenPath := writeTokenFile(t)

	code, refreshOutput, stderr := runCommand(commandArgs(tokenPath, "auth", "refresh"), transport, cli.Dependencies{}, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("auth refresh exit code = %d, stderr = %s", code, stderr)
	}

	for _, token := range []string{fixtureRotatedAccess, fixtureRotatedRefresh, fixtureRefresh} {
		if strings.Contains(refreshOutput, token) {
			t.Errorf("auth refresh output exposed token %q", token)
		}
	}

	var refreshed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, &refreshed)
	if err != nil {
		t.Fatal(err)
	}

	if refreshed.AccessToken != "synthetic-rotated-access" || refreshed.RefreshToken != "synthetic-rotated-refresh" {
		t.Fatalf("refreshed token file = %+v", refreshed)
	}

	exportPath := filepath.Join(t.TempDir(), "exported-tokens.json")
	args := commandArgs(tokenPath, "auth", "export", "--output", exportPath)

	code, exportOutput, stderr := runCommand(args, transport, cli.Dependencies{}, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("auth export exit code = %d, stderr = %s", code, stderr)
	}

	for _, token := range []string{"synthetic-rotated-access", "synthetic-rotated-refresh"} {
		if strings.Contains(exportOutput, token) {
			t.Errorf("auth export output exposed token %q", token)
		}
	}

	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(exported, []byte("synthetic-rotated-access")) || !bytes.Contains(exported, []byte("synthetic-rotated-refresh")) {
		t.Fatal("explicit token export omitted the rotated pair")
	}

	if err := transport.verifyConsumed(); err != nil {
		t.Fatal(err)
	}
}

func TestEventsCancellationStopsMQTTAndClosesSession(t *testing.T) {
	t.Parallel()
	fixtures := loadFixtures(t)
	transport := &pairedTransport{pairs: fixtures.Events}
	tokenPath := writeTokenFile(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	output := &cancelAfterEvents{cancel: cancel, want: 2}

	var (
		stderr bytes.Buffer
		broker *fakeMQTTClient
	)

	dependencies := cli.Dependencies{
		HTTPTransport: transport,
		MQTTClientFactory: func(options *mqtt.ClientOptions) mqtt.Client {
			broker = &fakeMQTTClient{options: options}

			return broker
		},
	}
	args := commandArgs(tokenPath, "events", "watch", "--home", "synthetic-home-01", "--duration", "3s")

	code := cli.Run(ctx, args, strings.NewReader(""), output, &stderr, dependencies)
	if code != 130 {
		t.Fatalf("events watch exit code = %d, want 130; stderr = %s", code, stderr.String())
	}

	if output.events != 2 {
		t.Fatalf("observed %d event records, want two", output.events)
	}

	if broker == nil || !broker.disconnected.Load() {
		t.Fatal("events watch did not disconnect the MQTT client")
	}

	if !strings.Contains(output.String(), "account_event") && !strings.Contains(output.String(), "device_state_change") {
		t.Fatalf("event output = %s", output.String())
	}

	for _, secret := range []string{"synthetic-password", "synthetic-user", "mqtt.example.invalid"} {
		if strings.Contains(output.String()+stderr.String(), secret) {
			t.Errorf("event command output exposed %q", secret)
		}
	}

	err := transport.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

type cancelAfterEvents struct {
	bytes.Buffer

	cancel func()
	want   int
	events int
}

func (writer *cancelAfterEvents) Write(data []byte) (int, error) {
	written, err := writer.Buffer.Write(data)
	if err != nil {
		return written, fmt.Errorf("write event output buffer: %w", err)
	}

	if bytes.Contains(data, []byte(`"type"`)) {
		writer.events++
		if writer.events >= writer.want {
			writer.cancel()
		}
	}

	return written, nil
}

type fakeMQTTClient struct {
	mqtt.Client

	options      *mqtt.ClientOptions
	connected    atomic.Bool
	disconnected atomic.Bool
}

func (client *fakeMQTTClient) IsConnected() bool { return client.connected.Load() }

func (client *fakeMQTTClient) Connect() mqtt.Token { //nolint:ireturn // Paho's interface requires an MQTT token result.
	client.connected.Store(true)

	if client.options.OnConnect != nil {
		client.options.OnConnect(client)
	}

	return completedToken{}
}

func (client *fakeMQTTClient) Subscribe( //nolint:ireturn // Paho's interface requires an MQTT token result.
	topic string,
	_ byte,
	_ mqtt.MessageHandler,
) mqtt.Token {
	if client.options.DefaultPublishHandler != nil {
		client.options.DefaultPublishHandler(client, testMQTTMessage{
			topic:   topic,
			payload: []byte(`{"protocol":4,"data":{"devId":"synthetic-device-01","status":[{"code":"switch_1","value":true}]}}`),
		})
	}

	return completedToken{}
}

func (*fakeMQTTClient) Unsubscribe(...string) mqtt.Token { //nolint:ireturn // Paho's interface requires an MQTT token result.
	return completedToken{}
}

func (client *fakeMQTTClient) Disconnect(uint) {
	client.connected.Store(false)
	client.disconnected.Store(true)
}

type completedToken struct{}

func (completedToken) Wait() bool                     { return true }
func (completedToken) WaitTimeout(time.Duration) bool { return true }
func (completedToken) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)

	return done
}
func (completedToken) Error() error { return nil }

type testMQTTMessage struct {
	topic   string
	payload []byte
}

func (message testMQTTMessage) Duplicate() bool   { return false }
func (message testMQTTMessage) Qos() byte         { return 0 }
func (message testMQTTMessage) Retained() bool    { return false }
func (message testMQTTMessage) Topic() string     { return message.topic }
func (message testMQTTMessage) MessageID() uint16 { return 0 }
func (message testMQTTMessage) Payload() []byte   { return message.payload }
func (testMQTTMessage) Ack()                      {}
