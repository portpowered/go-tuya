package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/portpowered/go-tuya/pkg/tuya"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/terminal"
	"golang.org/x/term"
)

type authQRResult struct {
	ScanRequired bool   `json:"scan_required"`
	PendingFile  string `json:"pending_file"`
}

type authPollResult struct {
	Authenticated bool       `json:"authenticated"`
	Pending       bool       `json:"pending"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	TokenFile     string     `json:"token_file,omitempty"`
}

type authRefreshResult struct {
	Refreshed bool       `json:"refreshed"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	TokenFile string     `json:"token_file"`
}

type authImportResult struct {
	Imported  bool   `json:"imported"`
	TokenFile string `json:"token_file"`
}

type authExportResult struct {
	Exported bool   `json:"exported"`
	Path     string `json:"path"`
}

type authLogoutResult struct {
	LoggedOut bool `json:"logged_out"`
}

func authCommand(ctx context.Context, args []string, config settings, input io.Reader, out, errOut io.Writer, deps Dependencies) error {
	if len(args) == 0 {
		return usageError("auth requires qr, poll, status, refresh, import, export, or logout")
	}

	switch args[0] {
	case "qr":
		return authQR(ctx, args[1:], config, input, out, errOut, deps)
	case "poll":
		return authPoll(ctx, args[1:], config, out, deps)
	case commandStatus:
		return authStatus(args[1:], config, out)
	case "refresh":
		return authRefresh(ctx, args[1:], config, out, deps)
	case "import":
		return authImport(args[1:], config, input, out)
	case "export":
		return authExport(args[1:], config, out)
	case "logout":
		return authLogout(args[1:], config, out)
	default:
		return usageError("unknown auth command")
	}
}

func authQR(ctx context.Context, args []string, config settings, input io.Reader, out, errOut io.Writer, deps Dependencies) error {
	flags := flag.NewFlagSet("auth qr", flag.ContinueOnError)
	accessCodeFile := flags.String("access-code-file", "", "read the Tuya access code from a file")
	schema := flags.String("schema", tuya.AuthenticationSchema, "Tuya authentication schema")

	rest, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth qr [--access-code-file path] [--schema name]")
	if err != nil || help {
		return err
	}

	if len(rest) != 0 {
		return usageError("auth qr does not accept positional arguments")
	}

	accessCode, err := readAccessCode(*accessCodeFile, input, errOut)
	if err != nil {
		return err
	}

	login, err := generateLoginQR(ctx, config, deps, accessCode, *schema)
	if err != nil {
		return err
	}

	if login.Code == "" || login.QrFormattedCode == "" {
		return newCommandError("provider returned an incomplete QR login response", "protocol")
	}

	err = writeProtectedJSON(config.pendingFile, pendingLogin{
		LoginCode: login.Code,
		UserCode:  accessCode,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	return displayLoginQR(config, out, errOut, deps, login.QrFormattedCode)
}

func displayLoginQR(config settings, out, errOut io.Writer, deps Dependencies, value string) error {
	renderer := deps.RenderQRCode
	if renderer == nil {
		renderer = renderTerminalQRCode
	}

	qrOutput := out
	if config.jsonOutput {
		qrOutput = errOut
	}

	err := renderer(qrOutput, value)
	if err != nil {
		return newCommandError("could not display login QR code", "terminal")
	}

	if config.jsonOutput {
		return writeJSON(out, authQRResult{ScanRequired: true, PendingFile: config.pendingFile})
	}

	_, err = fmt.Fprintf(
		out,
		"Scan the QR code with the Tuya or Smart Life app, then run `go-tuya --token-file %q auth poll`.\nPending login saved to %s\n",
		config.tokenFile,
		config.pendingFile,
	)
	if err != nil {
		return newCommandError("could not write command output", "output")
	}

	return nil
}

func generateLoginQR(ctx context.Context, config settings, deps Dependencies, accessCode, schema string) (tuya.LoginResponse, error) {
	client, err := newSDKClient(config, deps)
	if err != nil {
		var emptyResponse tuya.LoginResponse

		return emptyResponse, err
	}

	var noTokens tuya.Tokens

	session := client.NewSession(noTokens)
	defer closeSession(ctx, session)

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	login, err := session.AuthService.GenerateQrCodeForLogin(requestCtx, tuya.LoginRequest{
		AccessCode: accessCode,
		Schema:     schema,
	})
	if err != nil {
		var emptyResponse tuya.LoginResponse

		return emptyResponse, safeSDKError("generate login QR code", err)
	}

	return login, nil
}

func readAccessCode(path string, input io.Reader, prompt io.Writer) (string, error) {
	if path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // The caller explicitly selected a local credential input.
		if err != nil {
			return "", newCommandError("could not read access code file", "filesystem")
		}

		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", newCommandError("access code file is empty", "credentials")
		}

		return value, nil
	}

	if value := strings.TrimSpace(os.Getenv("TUYA_ACCESS_CODE")); value != "" {
		return value, nil
	}

	return readPromptAccessCode(input, prompt)
}

func readPromptAccessCode(input io.Reader, prompt io.Writer) (string, error) {
	_, _ = fmt.Fprint(prompt, "Tuya app access code: ")

	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return readHiddenAccessCode(file, prompt)
	}

	return readPlainAccessCode(input)
}

func readHiddenAccessCode(file *os.File, prompt io.Writer) (string, error) {
	value, err := term.ReadPassword(int(file.Fd()))
	_, _ = fmt.Fprintln(prompt)

	if err != nil {
		return "", newCommandError("could not read access code", "credentials")
	}

	accessCode := strings.TrimSpace(string(value))
	if accessCode == "" {
		return "", newCommandError("access code is required", "credentials")
	}

	return accessCode, nil
}

func readPlainAccessCode(input io.Reader) (string, error) {
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", newCommandError("could not read access code", "credentials")
	}

	accessCode := strings.TrimSpace(line)
	if accessCode == "" {
		return "", newCommandError("access code is required", "credentials")
	}

	return accessCode, nil
}

func authPoll(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	waitDuration, interval, err := parsePollOptions(args, out)
	if err != nil {
		return err
	}

	pending, err := loadPendingLogin(config.pendingFile)
	if err != nil {
		return err
	}

	client, err := newSDKClient(config, deps)
	if err != nil {
		return err
	}

	var noTokens tuya.Tokens

	session := client.NewSession(noTokens)
	defer closeSession(ctx, session)

	pollCtx := ctx

	if waitDuration > 0 {
		var cancel context.CancelFunc

		pollCtx, cancel = context.WithTimeout(ctx, waitDuration)
		defer cancel()
	}

	return pollForApproval(ctx, pollCtx, session, pending, waitDuration, interval, config, out)
}

func parsePollOptions(args []string, out io.Writer) (time.Duration, time.Duration, error) {
	flags := flag.NewFlagSet("auth poll", flag.ContinueOnError)
	waitDuration := flags.Duration("wait", 0, "keep polling until approved or this duration elapses")
	interval := flags.Duration("interval", defaultPollInterval, "time between approval checks")

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth poll [--wait duration] [--interval duration]")
	if err != nil || help {
		return 0, 0, err
	}

	if len(flags.Args()) != 0 || *waitDuration < 0 || *interval <= 0 {
		return 0, 0, usageError("auth poll requires valid wait and interval durations")
	}

	return *waitDuration, *interval, nil
}

func loadPendingLogin(path string) (pendingLogin, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The path is the explicit local pending-login file.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			var emptyPending pendingLogin

			return emptyPending, newCommandError("no pending QR login; run auth qr first", "credentials")
		}

		var emptyPending pendingLogin

		return emptyPending, newCommandError("could not read pending login", "filesystem")
	}

	var pending pendingLogin
	if json.Unmarshal(data, &pending) != nil || pending.LoginCode == "" || pending.UserCode == "" {
		var emptyPending pendingLogin

		return emptyPending, newCommandError("pending login file is invalid", "credentials")
	}

	return pending, nil
}

func pollForApproval(
	ctx, pollCtx context.Context,
	session *tuya.Session,
	pending pendingLogin,
	waitDuration, interval time.Duration,
	config settings,
	out io.Writer,
) error {
	for {
		requestCtx, cancel := context.WithTimeout(pollCtx, config.requestTimeout)
		response, err := session.AuthService.ValidateLoginCode(requestCtx, tuya.ValidateLoginCodeRequest{
			Request:   newRequest(),
			LoginCode: pending.LoginCode,
			UserCode:  pending.UserCode,
		})

		cancel()

		if err == nil {
			return saveValidatedLogin(config, response, out)
		}

		if !isPendingLoginError(err) {
			return safeSDKError("poll QR login", err)
		}

		keepPolling, waitErr := waitForLoginRetry(ctx, pollCtx, waitDuration, interval, config, out)
		if waitErr != nil || !keepPolling {
			return waitErr
		}
	}
}

func saveValidatedLogin(config settings, response tuya.ValidateLoginCodeResponse, out io.Writer) error {
	if response.AccessToken == "" || response.RefreshToken == "" {
		return newCommandError("provider returned incomplete tokens", "protocol")
	}

	cloudURL := config.cloudAPIURL
	if cloudURL == "" && response.Endpoint != "" && safeHTTPBase(response.Endpoint) == nil {
		cloudURL = strings.TrimRight(response.Endpoint, "/")
	}

	expireTime := int64(0)
	if response.ExpireTime > 0 {
		expireTime = time.Now().Add(time.Duration(response.ExpireTime) * time.Second).UnixMilli()
	}

	document := tokenDocument{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ExpireTime:   expireTime,
		CloudAPIURL:  cloudURL,
	}

	err := saveTokenDocument(config.tokenFile, document)
	if err != nil {
		return err
	}

	err = removeIfPresent(config.pendingFile)
	if err != nil {
		return err
	}

	result := authPollResult{
		Authenticated: true,
		Pending:       false,
		ExpiresAt:     expiredAt(expireTime),
		TokenFile:     config.tokenFile,
	}

	return emit(out, config.jsonOutput, result, "Login approved; tokens saved to "+config.tokenFile)
}

func isPendingLoginError(err error) bool {
	var sdkErr *tuya.ClientError

	return errors.As(err, &sdkErr) && sdkErr.Kind == tuya.ErrorProvider
}

func waitForLoginRetry(ctx, pollCtx context.Context, waitDuration, interval time.Duration, config settings, out io.Writer) (bool, error) {
	if waitDuration == 0 {
		err := emit(out, config.jsonOutput, pendingPollResult(), "Login is still pending; scan the QR code and run auth poll again.")

		return false, err
	}

	if pollCtx.Err() != nil {
		if ctx.Err() != nil {
			return false, newCommandError("login polling interrupted", "interrupted", exitCodeInterrupted)
		}

		err := emit(out, config.jsonOutput, pendingPollResult(), "Login is still pending; run auth poll again.")

		return false, err
	}

	err := waitContext(pollCtx, interval)
	if err != nil {
		if ctx.Err() != nil {
			return false, newCommandError("login polling interrupted", "interrupted", exitCodeInterrupted)
		}

		err = emit(out, config.jsonOutput, pendingPollResult(), "Login is still pending; run auth poll again.")

		return false, err
	}

	return true, nil
}

func pendingPollResult() authPollResult {
	return authPollResult{
		Authenticated: false,
		Pending:       true,
		ExpiresAt:     nil,
		TokenFile:     "",
	}
}

func authStatus(args []string, config settings, out io.Writer) error {
	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth status")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 {
		return usageError("auth status does not accept arguments")
	}

	document, present, err := optionalTokenDocument(config)
	if err != nil {
		return err
	}

	result := tokenStatus{
		Authenticated:   present,
		HasRefreshToken: present && document.RefreshToken != "",
		ExpiresAt:       nil,
		TokenFile:       "",
	}
	if present {
		result.TokenFile = config.tokenFile
		result.ExpiresAt = expiredAt(document.ExpireTime)
	}

	human := "No tokens are configured."
	if present {
		human = "Tokens are available; token values are hidden."
		if result.ExpiresAt != nil {
			human += " Expiry: " + result.ExpiresAt.Format(time.RFC3339) + "."
		}
	}

	return emit(out, config.jsonOutput, result, human)
}

func authRefresh(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	flags := flag.NewFlagSet("auth refresh", flag.ContinueOnError)

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth refresh")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 {
		return usageError("auth refresh does not accept arguments")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	var response tuya.RefreshTokenResponse

	err = withSession(ctx, config, document, deps, func(session *tuya.Session) error {
		requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
		defer cancel()

		refreshed, refreshErr := session.AuthService.RefreshToken(requestCtx, tuya.RefreshTokenRequest{
			Request:      newRequest(),
			RefreshToken: document.RefreshToken,
		})
		if refreshErr != nil {
			return safeSDKError("refresh Tuya tokens", refreshErr)
		}

		response = refreshed

		return nil
	})
	if err != nil {
		return err
	}

	return persistRefreshedTokens(response, document, config, out)
}

func persistRefreshedTokens(response tuya.RefreshTokenResponse, previous tokenDocument, config settings, out io.Writer) error {
	if response.AccessToken == "" || response.RefreshToken == "" {
		return newCommandError("provider returned incomplete refreshed tokens", "protocol")
	}

	updated := tokenDocument{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ExpireTime:   response.ExpireTime,
		CloudAPIURL:  previous.CloudAPIURL,
	}

	err := saveTokenDocument(config.tokenFile, updated)
	if err != nil {
		return err
	}

	result := authRefreshResult{
		Refreshed: true,
		ExpiresAt: expiredAt(updated.ExpireTime),
		TokenFile: config.tokenFile,
	}

	return emit(out, config.jsonOutput, result, "Tokens refreshed and saved to "+config.tokenFile)
}

func authImport(args []string, config settings, input io.Reader, out io.Writer) error {
	flags := flag.NewFlagSet("auth import", flag.ContinueOnError)
	inputFile := flags.String("file", "", "read token JSON from a file")
	readStdin := flags.Bool("stdin", false, "read token JSON from stdin")

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth import (--file path|--stdin)")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 || (*inputFile != "") == *readStdin {
		return usageError("choose exactly one of --file or --stdin")
	}

	var reader = input

	if *inputFile != "" {
		file, openErr := os.Open(*inputFile)
		if openErr != nil {
			return newCommandError("could not open token input file", "filesystem")
		}

		defer func() { _ = file.Close() }()

		reader = file
	}

	document, err := readJSONInput(reader)
	if err != nil {
		return err
	}

	err = saveTokenDocument(config.tokenFile, document)
	if err != nil {
		return err
	}

	return emit(out, config.jsonOutput, authImportResult{Imported: true, TokenFile: config.tokenFile}, "Tokens imported to "+config.tokenFile)
}

func authExport(args []string, config settings, out io.Writer) error {
	flags := flag.NewFlagSet("auth export", flag.ContinueOnError)
	outputPath := flags.String("output", "", "write tokens to a protected file")

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth export --output path")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 || *outputPath == "" {
		return usageError("auth export requires --output path")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	err = saveTokenDocument(*outputPath, document)
	if err != nil {
		return err
	}

	return emit(out, config.jsonOutput, authExportResult{Exported: true, Path: *outputPath}, "Tokens exported to "+*outputPath)
}

func authLogout(args []string, config settings, out io.Writer) error {
	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)

	_, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] auth logout")
	if err != nil || help {
		return err
	}

	if len(flags.Args()) != 0 {
		return usageError("auth logout does not accept arguments")
	}

	err = removeIfPresent(config.tokenFile)
	if err != nil {
		return err
	}

	err = removeIfPresent(config.pendingFile)
	if err != nil {
		return err
	}

	return emit(out, config.jsonOutput, authLogoutResult{LoggedOut: true}, "Saved tokens and pending QR login removed.")
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return newCommandError("could not remove credential file", "filesystem")
}

func closeSession(ctx context.Context, session *tuya.Session) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()

	_ = session.Close(closeCtx)
	session.HTTPClient.CloseIdleConnections()
}

func parseCommandFlags(flags *flag.FlagSet, args []string, out io.Writer, usageText string) ([]string, bool, error) {
	flags.SetOutput(io.Discard)

	err := flags.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprintln(out, usageText)

			return nil, true, nil
		}

		return nil, false, usageError("invalid command flag")
	}

	return flags.Args(), false, nil
}

func usageError(message string) error {
	return newCommandError(message, "usage", exitCodeUsage)
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for login approval: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func renderTerminalQRCode(_ io.Writer, value string) error {
	qr, err := qrcode.New(value)
	if err != nil {
		return fmt.Errorf("create terminal QR code: %w", err)
	}

	err = qr.Save(terminal.New())
	if err != nil {
		return fmt.Errorf("render terminal QR code: %w", err)
	}

	return nil
}
