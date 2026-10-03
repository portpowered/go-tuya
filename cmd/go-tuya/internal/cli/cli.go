// Package cli implements the standalone Tuya command-line interface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/portpowered/go-tuya/pkg/tuya"
)

const (
	defaultRequestTimeout = 30 * time.Second
	defaultEventDuration  = 60 * time.Second
	defaultPollInterval   = 2 * time.Second
	exitCodeUsage         = 2
	exitCodeInterrupted   = 130
	schemeHTTP            = "http"
	schemeHTTPS           = "https"
	commandStatus         = "status"
)

var (
	errInvalidHTTPBase   = errors.New("invalid HTTP base")
	errInsecureHTTPBase  = errors.New("insecure HTTP bases must use loopback")
	errInvalidProxy      = errors.New("invalid HTTP proxy URL")
	errInvalidMQTTBroker = errors.New("invalid MQTT broker URL")
)

// Dependencies supplies optional offline seams for command tests and local
// brokers. The CLI itself still calls provider operations through the public SDK.
type Dependencies struct {
	HTTPTransport     http.RoundTripper
	MQTTClientFactory tuya.MQTTClientFactory
	RenderQRCode      func(io.Writer, string) error
}

type settings struct {
	tokenFile         string
	pendingFile       string
	cloudAPIURL       string
	authenticationURL string
	clientID          string
	httpProxy         string
	mqttBrokerURL     string
	requestTimeout    time.Duration
	jsonOutput        bool
}

type commandError struct {
	message  string
	kind     string
	exitCode int
}

func (e *commandError) Error() string { return e.message }

func newCommandError(message, kind string, exitCodes ...int) *commandError {
	exitCode := 0
	if len(exitCodes) == 1 {
		exitCode = exitCodes[0]
	}

	return &commandError{message: message, kind: kind, exitCode: exitCode}
}

func usage(out io.Writer) {
	const text = `Usage: go-tuya [global flags] <command>

Commands:
  auth qr [--access-code-file path]     Generate a Tuya login QR code
  auth poll [--wait duration]           Poll QR login and save approved tokens
  auth status                           Show saved token status without secrets
  auth refresh                          Explicitly refresh and save tokens
  auth import (--file path|--stdin)     Import tokens from JSON
  auth export --output path             Export tokens to a protected file
  auth logout                           Remove saved tokens and pending login
  homes list                            List homes
  devices list [--home id]              List devices in homes
  devices status <device-id>            Read a device's state
  devices spec <device-id>              Read a device's specification
  device command (--value-file path|--value-stdin) <id> <code>
                                        Send an explicit device command
  events watch [--duration duration]    Watch account and device events

Global flags:
  --token-file path       Token file (default: user config directory)
  --pending-file path     Pending QR login file (default: user config directory)
  --cloud-api-url url     Cloud API base URL override
  --auth-url url          Authentication API base URL override
  --client-id id          Tuya application client ID override
  --http-proxy url        HTTP proxy for local sandbox testing
  --mqtt-broker-url url   MQTT broker override for sandbox testing
  --request-timeout time  Per-request deadline (default: 30s)
  --json                  Emit JSON results and JSON Lines events

Credentials come from the protected token file, Tuya environment variables,
stdin, or an explicit input file. Secret values are never command arguments.`

	_, _ = fmt.Fprintln(out, text)
}

// Run executes the command and returns its process exit code. It accepts streams
// explicitly so commands can be exercised offline without changing process state.
func Run(ctx context.Context, args []string, input io.Reader, out, errOut io.Writer, deps Dependencies) int {
	configDir, err := userConfigDir()
	if err != nil {
		return report(errOut, err, hasFlag(args, "--json"))
	}

	config := settings{
		tokenFile:         filepath.Join(configDir, "go-tuya", "tokens.json"),
		pendingFile:       filepath.Join(configDir, "go-tuya", "pending-login.json"),
		cloudAPIURL:       "",
		authenticationURL: "",
		clientID:          "",
		httpProxy:         "",
		mqttBrokerURL:     "",
		requestTimeout:    defaultRequestTimeout,
		jsonOutput:        false,
	}

	commandArgs, code, finished := parseGlobalOptions(args, &config, out, errOut)
	if finished {
		return code
	}

	if config.requestTimeout <= 0 {
		return report(errOut, newCommandError("request timeout must be positive", "usage", exitCodeUsage), config.jsonOutput)
	}

	if len(commandArgs) == 0 || commandArgs[0] == "help" {
		usage(out)

		if len(commandArgs) == 0 {
			return exitCodeUsage
		}

		return 0
	}

	err = validateSettings(config)
	if err != nil {
		return report(errOut, err, config.jsonOutput)
	}

	code = dispatch(ctx, commandArgs, config, input, out, errOut, deps)

	return code
}

func parseGlobalOptions(args []string, config *settings, out, errOut io.Writer) ([]string, int, bool) {
	flags := flag.NewFlagSet("go-tuya", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&config.tokenFile, "token-file", config.tokenFile, "token file")
	flags.StringVar(&config.pendingFile, "pending-file", config.pendingFile, "pending QR login file")
	flags.StringVar(&config.cloudAPIURL, "cloud-api-url", "", "cloud API base URL override")
	flags.StringVar(&config.authenticationURL, "auth-url", "", "authentication API base URL override")
	flags.StringVar(&config.clientID, "client-id", "", "Tuya application client ID override")
	flags.StringVar(&config.httpProxy, "http-proxy", "", "HTTP proxy URL for sandbox testing")
	flags.StringVar(&config.mqttBrokerURL, "mqtt-broker-url", "", "MQTT broker URL override for sandbox testing")
	flags.DurationVar(&config.requestTimeout, "request-timeout", defaultRequestTimeout, "per-request timeout")
	flags.BoolVar(&config.jsonOutput, "json", false, "emit JSON results")

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		usage(out)

		return nil, 0, true
	}

	if err != nil {
		code := report(errOut, newCommandError("invalid global flag", "usage", exitCodeUsage), config.jsonOutput)

		return nil, code, true
	}

	return flags.Args(), 0, false
}

func dispatch(ctx context.Context, args []string, config settings, input io.Reader, out, errOut io.Writer, deps Dependencies) int {
	var err error

	switch args[0] {
	case "auth":
		err = authCommand(ctx, args[1:], config, input, out, errOut, deps)
	case "homes":
		err = homesCommand(ctx, args[1:], config, out, deps)
	case "devices":
		err = devicesCommand(ctx, args[1:], config, out, deps)
	case "device":
		err = deviceCommand(ctx, args[1:], config, input, out, deps)
	case "events":
		err = eventsCommand(ctx, args[1:], config, out, deps)
	default:
		usage(out)

		err = newCommandError("unknown command", "usage", exitCodeUsage)
	}

	if err == nil {
		return 0
	}

	return report(errOut, err, config.jsonOutput)
}

func validateSettings(config settings) error {
	if config.tokenFile == "" || config.pendingFile == "" {
		return newCommandError("token and pending file paths are required", "usage", exitCodeUsage)
	}

	err := validateEndpointOverride("cloud API URL", config.cloudAPIURL)
	if err != nil {
		return err
	}

	err = validateEndpointOverride("authentication URL", config.authenticationURL)
	if err != nil {
		return err
	}

	err = validateProxyOverride(config.httpProxy)
	if err != nil {
		return err
	}

	err = validateMQTTOverride(config.mqttBrokerURL)
	if err != nil {
		return err
	}

	return nil
}

func validateEndpointOverride(name, value string) error {
	if value == "" {
		return nil
	}

	if safeHTTPBase(value) != nil {
		return newCommandError("invalid "+name+" override", "usage", exitCodeUsage)
	}

	return nil
}

func validateProxyOverride(value string) error {
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != schemeHTTPS && parsed.Scheme != schemeHTTP) {
		return newCommandError(errInvalidProxy.Error(), "usage", exitCodeUsage)
	}

	if parsed.Scheme == schemeHTTP && !isLoopbackHost(parsed.Hostname()) {
		return newCommandError("insecure HTTP proxies must use loopback", "usage", exitCodeUsage)
	}

	return nil
}

func validateMQTTOverride(value string) error {
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || !supportedMQTTScheme(parsed.Scheme) {
		return newCommandError(errInvalidMQTTBroker.Error(), "usage", exitCodeUsage)
	}

	if (parsed.Scheme == schemeTCP || parsed.Scheme == schemeWS) && !isLoopbackHost(parsed.Hostname()) {
		return newCommandError("insecure MQTT broker overrides must use loopback", "usage", exitCodeUsage)
	}

	return nil
}

func safeHTTPBase(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errInvalidHTTPBase
	}

	if parsed.Scheme == schemeHTTPS {
		return nil
	}

	if parsed.Scheme == schemeHTTP && isLoopbackHost(parsed.Hostname()) {
		return nil
	}

	return errInsecureHTTPBase
}

func supportedMQTTScheme(scheme string) bool {
	switch scheme {
	case "tcp", "ssl", "ws", "wss":
		return true
	default:
		return false
	}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	address := net.ParseIP(host)

	return address != nil && address.IsLoopback()
}

func newSDKClient(config settings, deps Dependencies) (*tuya.Client, error) {
	transport, err := configuredHTTPTransport(config.httpProxy, deps.HTTPTransport)
	if err != nil {
		return nil, err
	}

	var httpClient http.Client

	httpClient.Transport = transport
	httpClient.Timeout = config.requestTimeout

	options := []tuya.Option{tuya.WithHTTPClient(&httpClient)}
	if config.clientID != "" {
		options = append(options, tuya.WithClientID(config.clientID))
	}

	if config.authenticationURL != "" {
		options = append(options, tuya.WithAuthenticationURL(config.authenticationURL))
	}

	if config.cloudAPIURL != "" {
		options = append(options, tuya.WithCloudAPIURL(config.cloudAPIURL))
	}

	factory, err := configuredMQTTFactory(config.mqttBrokerURL, deps.MQTTClientFactory)
	if err != nil {
		return nil, err
	}

	if factory != nil {
		options = append(options, tuya.WithMQTTClientFactory(factory))
	}

	client, err := tuya.NewClient(options...)
	if err != nil {
		return nil, safeSDKError("configure Tuya client", err)
	}

	return client, nil
}

func configuredHTTPTransport(proxy string, injected http.RoundTripper) (http.RoundTripper, error) {
	if injected != nil || proxy == "" {
		return injected, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, newCommandError(errInvalidProxy.Error(), "usage", exitCodeUsage)
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, newCommandError("default HTTP transport is unavailable", "internal")
	}

	proxyTransport := defaultTransport.Clone()
	proxyTransport.Proxy = http.ProxyURL(proxyURL)

	return proxyTransport, nil
}

func configuredMQTTFactory(value string, injected tuya.MQTTClientFactory) (tuya.MQTTClientFactory, error) {
	if injected != nil || value == "" {
		return injected, nil
	}

	brokerURL, err := url.Parse(value)
	if err != nil {
		return nil, newCommandError(errInvalidMQTTBroker.Error(), "usage", exitCodeUsage)
	}

	factory := func(options *mqtt.ClientOptions) mqtt.Client {
		options.Servers = []*url.URL{brokerURL}

		return mqtt.NewClient(options)
	}

	return factory, nil
}

func userConfigDir() (string, error) {
	path, err := os.UserConfigDir()
	if err != nil {
		return "", newCommandError("could not determine configuration directory", "filesystem")
	}

	return path, nil
}

func hasFlag(args []string, flagName string) bool {
	for _, arg := range args {
		if arg == flagName || strings.HasPrefix(arg, flagName+"=") {
			return true
		}
	}

	return false
}

func report(out io.Writer, err error, jsonOutput bool) int {
	message := "command failed"
	kind := "internal"
	exitCode := 1

	var commandErr *commandError
	if errors.As(err, &commandErr) {
		message = commandErr.message

		if commandErr.kind != "" {
			kind = commandErr.kind
		}

		if commandErr.exitCode != 0 {
			exitCode = commandErr.exitCode
		}
	}

	if jsonOutput {
		_ = writeJSON(out, errorResult{Error: message, Kind: kind})
	} else {
		_, _ = fmt.Fprintf(out, "go-tuya: %s\n", message)
	}

	return exitCode
}

type errorResult struct {
	Error string `json:"error"`
	Kind  string `json:"kind"`
}

func safeSDKError(action string, err error) error {
	kind := "provider"

	var sdkError *tuya.ClientError

	if errors.As(err, &sdkError) && sdkError.Kind != "" {
		kind = string(sdkError.Kind)
	}

	return newCommandError(action+" failed", kind)
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("write JSON result: %w", err)
	}

	return nil
}

func emit(out io.Writer, jsonOutput bool, value any, human string) error {
	if jsonOutput {
		return writeJSON(out, value)
	}

	_, err := fmt.Fprintln(out, human)
	if err != nil {
		return fmt.Errorf("write command result: %w", err)
	}

	return nil
}
