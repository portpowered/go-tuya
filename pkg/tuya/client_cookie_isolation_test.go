package tuya //nolint:testpackage // Verifies private reusable-client snapshots and account-session state.

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type cookieIsolationExpectation struct {
	token   string
	cookies map[string]string
}

func TestNewClientRejectsSharedHTTPClientCookieJar(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create synthetic cookie jar: %v", err)
	}

	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errTestUnexpectedRequest
	})
	injectedClient := &http.Client{CheckRedirect: nil, Transport: transport, Jar: jar, Timeout: 0}

	client, err := newSyntheticClient(WithHTTPClient(injectedClient))
	if client != nil || err == nil {
		t.Fatalf("newSyntheticClient() = (%v, %v), want nil client and cookie-jar error", client, err)
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorInvalidOperation {
		t.Fatalf("NewClient error = %T %v, want inspectable invalid-operation ClientError", err, err)
	}

	if !errors.Is(err, errHTTPClientCookieJar) {
		t.Fatalf("NewClient error = %v, want the CookieJar configuration cause", err)
	}

	if !sameTransport(injectedClient.Transport, transport) || injectedClient.Jar != jar {
		t.Fatal("NewClient mutated the rejected caller HTTP client")
	}
}

func TestSessionsIsolateCookieJarsWithInjectedHTTPClient(t *testing.T) {
	t.Parallel()

	const (
		origin     = "https://cloud.synthetic.test"
		requestURL = origin + "/v1.0/devices"
	)

	target := syntheticCookieTarget(t, requestURL)

	wantRequests := []cookieIsolationExpectation{
		{token: clientFixtureSyntheticAccessOne, cookies: map[string]string{clientFixtureSyntheticAccountCookie: clientFixtureSyntheticAccountOne}},
		{token: clientFixtureSyntheticAccessTwo, cookies: map[string]string{clientFixtureSyntheticAccountCookie: clientFixtureSyntheticAccountTwo}},
		{token: clientFixtureSyntheticAccessOne, cookies: map[string]string{
			clientFixtureSyntheticAccountCookie: clientFixtureSyntheticAccountOne,
			clientFixtureSyntheticServerCookie:  clientFixtureSyntheticAccountOne,
		}},
		{token: clientFixtureSyntheticAccessTwo, cookies: map[string]string{
			clientFixtureSyntheticAccountCookie: clientFixtureSyntheticAccountTwo,
			clientFixtureSyntheticServerCookie:  clientFixtureSyntheticAccountTwo,
		}},
	}

	requestCount := 0
	transport := newCookieIsolationTransport(t, requestURL, wantRequests, &requestCount)
	injectedClient := &http.Client{CheckRedirect: nil, Transport: transport, Jar: nil, Timeout: 13 * time.Second}

	client, err := newSyntheticClient(WithHTTPClient(injectedClient), WithCloudAPIURL(origin))
	if err != nil {
		t.Fatalf("newSyntheticClient() error = %v", err)
	}

	callerJar := setSyntheticCookie(t, target, clientFixtureSyntheticCallerCookie, clientFixtureSyntheticCallerValue)
	injectedClient.Jar = callerJar

	first := client.NewSession(Tokens{
		AccessToken: clientFixtureSyntheticAccessOne, RefreshToken: clientFixtureSyntheticRefreshOne, ExpireTime: 0,
	})
	second := client.NewSession(Tokens{
		AccessToken: clientFixtureSyntheticAccessTwo, RefreshToken: clientFixtureSyntheticRefreshTwo, ExpireTime: 0,
	})
	assertSessionHTTPClientSnapshots(t, injectedClient, client, first, second, transport)

	firstJar := setSyntheticCookie(t, target, clientFixtureSyntheticAccountCookie, clientFixtureSyntheticAccountOne)
	first.HTTPClient.Jar = firstJar

	secondJar := setSyntheticCookie(t, target, clientFixtureSyntheticAccountCookie, clientFixtureSyntheticAccountTwo)
	second.HTTPClient.Jar = secondJar
	assertSessionCookieJars(t, client, first, second, firstJar, secondJar)

	assertUnauthorizedSyntheticRequest(t, first)
	assertUnauthorizedSyntheticRequest(t, second)
	assertUnauthorizedSyntheticRequest(t, first)
	assertUnauthorizedSyntheticRequest(t, second)

	if requestCount != len(wantRequests) {
		t.Fatalf("synthetic request count = %d, want %d", requestCount, len(wantRequests))
	}

	assertCallerJarWasNotUsed(t, callerJar, target)
}

func assertSessionHTTPClientSnapshots(
	t *testing.T,
	injectedClient *http.Client,
	client *Client,
	first, second *Session,
	transport http.RoundTripper,
) {
	t.Helper()

	assertIndependentSessionHTTPClients(t, injectedClient, client, first, second)
	assertSessionClientsHaveNoJar(t, client, first, second)
	assertSessionTransportsPreserved(t, client, first, second, transport)
}

func assertIndependentSessionHTTPClients(t *testing.T, injectedClient *http.Client, client *Client, first, second *Session) {
	t.Helper()

	if client.options.httpClient == injectedClient || first.HTTPClient == injectedClient ||
		second.HTTPClient == injectedClient || first.HTTPClient == second.HTTPClient {
		t.Fatal("injected, reusable, and account HTTP clients were not independently snapshotted")
	}
}

func assertSessionClientsHaveNoJar(t *testing.T, client *Client, first, second *Session) {
	t.Helper()

	if client.options.httpClient.Jar != nil || first.HTTPClient.Jar != nil || second.HTTPClient.Jar != nil {
		t.Fatal("late caller CookieJar assignment reached an effective account HTTP client")
	}
}

func assertSessionTransportsPreserved(t *testing.T, client *Client, first, second *Session, transport http.RoundTripper) {
	t.Helper()

	if !sameTransport(client.options.httpClient.Transport, transport) ||
		!sameTransport(first.HTTPClient.Transport, transport) || !sameTransport(second.HTTPClient.Transport, transport) {
		t.Fatal("HTTP client snapshots did not preserve injected Transport identity")
	}
}

func assertSessionCookieJars(
	t *testing.T,
	client *Client,
	first, second *Session,
	firstJar, secondJar http.CookieJar,
) {
	t.Helper()

	if first.HTTPClient.Jar != firstJar || second.HTTPClient.Jar != secondJar ||
		client.options.httpClient.Jar != nil || first.HTTPClient.Jar == second.HTTPClient.Jar {
		t.Fatal("account CookieJar assignments were not isolated from each other and the reusable client")
	}
}

func newCookieIsolationTransport(
	t *testing.T,
	requestURL string,
	wantRequests []cookieIsolationExpectation,
	requestCount *int,
) roundTripFunc {
	t.Helper()

	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if *requestCount >= len(wantRequests) {
			t.Errorf("unexpected extra synthetic request: %s %s", request.Method, request.URL)

			return nil, errTestUnexpectedRequest
		}

		index := *requestCount
		assertSyntheticSessionRequest(t, request, requestURL, index, wantRequests[index])

		responseHeaders := make(http.Header)

		if index < 2 {
			cookieValue := wantRequests[index].cookies[clientFixtureSyntheticAccountCookie]
			responseHeaders.Set("Set-Cookie", clientFixtureSyntheticServerCookie+"="+cookieValue+"; Path=/; Secure; HttpOnly; SameSite=Strict")
		}

		*requestCount++

		responseBody := "synthetic unauthorized response"

		return &http.Response{
			Status:           "401 Unauthorized",
			StatusCode:       http.StatusUnauthorized,
			Proto:            "HTTP/1.1",
			ProtoMajor:       1,
			ProtoMinor:       1,
			Header:           responseHeaders,
			Body:             io.NopCloser(strings.NewReader(responseBody)),
			ContentLength:    int64(len(responseBody)),
			TransferEncoding: nil,
			Close:            false,
			Uncompressed:     false,
			Trailer:          nil,
			Request:          request,
			TLS:              nil,
		}, nil
	})
}

func assertSyntheticSessionRequest(
	t *testing.T,
	request *http.Request,
	wantURL string,
	index int,
	want cookieIsolationExpectation,
) {
	t.Helper()

	assertSyntheticRequestTarget(t, request, wantURL, index)
	assertSyntheticRequestFraming(t, request, index)
	assertSyntheticRequestHeaders(t, request, index, want.token)
	assertSyntheticRequestCookies(t, request, index, want.cookies)
}

func assertSyntheticRequestTarget(t *testing.T, request *http.Request, wantURL string, index int) {
	t.Helper()

	if request.Method != http.MethodGet || request.URL.String() != wantURL || request.URL.User != nil ||
		request.URL.Fragment != "" || request.URL.RawQuery != "" || request.Host != "cloud.synthetic.test" {
		t.Errorf("request %d target = %s %s (Host %q), want GET %s", index, request.Method, request.URL,
			request.Host, wantURL)
	}
}

func assertSyntheticRequestFraming(t *testing.T, request *http.Request, index int) {
	t.Helper()

	if request.Proto != "HTTP/1.1" || request.ProtoMajor != 1 || request.ProtoMinor != 1 ||
		request.Body != nil || request.GetBody != nil || request.ContentLength != 0 ||
		len(request.TransferEncoding) != 0 || request.Close {
		t.Errorf("request %d framing = proto %q (%d.%d), body=%v GetBody=%t length=%d transfer=%v close=%t",
			index, request.Proto, request.ProtoMajor, request.ProtoMinor, request.Body,
			request.GetBody != nil, request.ContentLength, request.TransferEncoding, request.Close)
	}
}

func assertSyntheticRequestHeaders(t *testing.T, request *http.Request, index int, wantToken string) {
	t.Helper()

	requestID := request.Header.Get("X-Requestid")
	timestamp := request.Header.Get("X-Time")
	signature := request.Header.Get("X-Sign")

	if len(requestID) != 36 || strings.Count(requestID, "-") != 4 {
		t.Errorf("request %d X-requestId = %q, want UUID-shaped value", index, requestID)
	}

	timeValue, timeErr := strconv.ParseInt(timestamp, 10, 64)
	if timeErr != nil || timestamp == "" || timeValue <= 0 {
		t.Errorf("request %d X-time = %q, want positive integer timestamp", index, timestamp)
	}

	decodedSignature, signatureErr := hex.DecodeString(signature)
	if signatureErr != nil || len(decodedSignature) != 32 {
		t.Errorf("request %d X-sign = %q, want 32-byte hex signature", index, signature)
	}

	gotHeaders := normalizeRequestHeaders(request.Header)
	wantHeaders := map[string][]string{
		"cookie":      request.Header.Values("Cookie"),
		"x-appkey":    {clientFixtureSyntheticClientID},
		"x-requestid": {requestID},
		"x-sid":       {""},
		"x-sign":      {signature},
		"x-time":      {timestamp},
		"x-token":     {wantToken},
	}

	if !reflect.DeepEqual(gotHeaders, wantHeaders) {
		t.Errorf("request %d headers = %#v, want %#v", index, gotHeaders, wantHeaders)
	}
}

func normalizeRequestHeaders(header http.Header) map[string][]string {
	result := make(map[string][]string, len(header))
	for name, values := range header {
		result[strings.ToLower(name)] = values
	}

	return result
}

func assertSyntheticRequestCookies(t *testing.T, request *http.Request, index int, want map[string]string) {
	t.Helper()

	got := make(map[string]string)
	for _, cookie := range request.Cookies() {
		got[cookie.Name] = cookie.Value
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("request %d cookies = %#v, want %#v", index, got, want)
	}
}

func assertUnauthorizedSyntheticRequest(t *testing.T, session *Session) {
	t.Helper()

	_, err := session.EncryptedClient.Get(
		context.Background(), "/v1.0/devices", nil,
		testOperationRequest{Request: Request{AuthorizationContext: nil, DoNotRefreshToken: false}},
	)

	var clientErr *ClientError
	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorUnauthorized {
		t.Fatalf("synthetic request error = %T %v, want unauthorized ClientError", err, err)
	}
}

func setSyntheticCookie(t *testing.T, target *url.URL, name, value string) *cookiejar.Jar {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create synthetic cookie jar: %v", err)
	}

	jar.SetCookies(target, []*http.Cookie{{
		Name: name, Value: value, Quoted: false, Path: "/", Domain: "", Expires: time.Time{}, RawExpires: "", MaxAge: 0,
		Secure: true, HttpOnly: true, Partitioned: false, SameSite: http.SameSiteStrictMode, Raw: "", Unparsed: nil,
	}})

	return jar
}

func assertCallerJarWasNotUsed(t *testing.T, callerJar *cookiejar.Jar, target *url.URL) {
	t.Helper()

	if hasSyntheticCookie(callerJar.Cookies(target), clientFixtureSyntheticServerCookie, clientFixtureSyntheticAccountOne) ||
		hasSyntheticCookie(callerJar.Cookies(target), clientFixtureSyntheticServerCookie, clientFixtureSyntheticAccountTwo) {
		t.Fatal("session responses mutated the caller's late-assigned CookieJar")
	}
}

func hasSyntheticCookie(cookies []*http.Cookie, name, want string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value == want {
			return true
		}
	}

	return false
}

func syntheticCookieTarget(t *testing.T, requestURL string) *url.URL {
	t.Helper()

	target, err := url.Parse(requestURL)
	if err != nil {
		t.Fatalf("parse synthetic request URL: %v", err)
	}

	return target
}

func sameTransport(first, second http.RoundTripper) bool {
	return reflect.TypeOf(first) == reflect.TypeOf(second) &&
		reflect.ValueOf(first).Pointer() == reflect.ValueOf(second).Pointer()
}
