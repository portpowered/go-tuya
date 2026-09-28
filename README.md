# go-tuya

[![Go version](https://img.shields.io/github/go-mod/go-version/portpowered/go-tuya)](go.mod)
[![CI](https://github.com/portpowered/go-tuya/actions/workflows/go.yml/badge.svg)](https://github.com/portpowered/go-tuya/actions/workflows/go.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://portpowered.github.io/go-tuya/coverage.json)](https://portpowered.github.io/go-tuya/coverage.html)
[![Release](https://img.shields.io/github/v/release/portpowered/go-tuya?display_name=tag)](https://github.com/portpowered/go-tuya/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/portpowered/go-tuya/pkg/tuya.svg)](https://pkg.go.dev/github.com/portpowered/go-tuya/pkg/tuya)
[![License](https://img.shields.io/github/license/portpowered/go-tuya)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-guides-blue)](https://portpowered.github.io/go-tuya/)

A Go client for the Tuya Device Sharing API. The public package exposes QR-code
authentication, regional cloud requests, home and device operations, command
submission, event listeners, capability conversion helpers, and camera RTC
stream controls. Applications provide and securely store credentials.

## Install

Requires Go 1.24 or later.

```sh
go get github.com/portpowered/go-tuya@v0.2.0
```

`v0.1.0` was the first release from the cleaned history. The package shape and
session API documented here are available in `v0.2.0`. Older version tags were
removed during credential-history cleanup.

Import the client package:

```go
import "github.com/portpowered/go-tuya/pkg/tuya"
```

## Supported surface

- QR-code login, login validation, and token refresh.
- Query homes, devices, device status, specifications, details, logs, and
  factory information. Update device names and functions; send device
  commands; manage device users and multi-outlet names; delete or reset
  devices; and query sub-devices.
- Subscribe to account and device events through the Tuya sharing message
  queue. Parse protocol 4 device-state events and protocol 20 management
  events.
- Allocate and manage camera RTC streams, when the device and account support
  the required Tuya flow.
- Convert selected device status codes to local capability values with
  `DeviceCapabilityMap`. The mapping table is intentionally limited to the
  categories and codes listed by `GetSupportedCategories` and
  `GetSupportedCodesForCategory`.

The client does not implement local-network device control. Tuya command codes
and supported properties vary by device; check the device specification
returned by the API before sending a command. The scene service and the
`Unload` placeholder do not currently provide implemented operations.

## Authentication and requests

`Client` stores reusable endpoint and transport configuration. Create a
`Session` for each account to hold its tokens, services, message queue, and
connection lifecycle. The application loads and stores credentials:

```go
import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/portpowered/go-tuya/pkg/tuya"
)

func listHomes(ctx context.Context, accessToken, refreshToken string, expiryMilliseconds int64) error {
    base, err := tuya.NewClient(
        tuya.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
        tuya.WithRegion(tuya.TuyaRegionUS),
    )
    if err != nil {
        return err
    }
    session := base.NewSession(tuya.Tokens{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpireTime:   expiryMilliseconds,
    })
    defer session.Close(ctx)

    homes, err := session.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
    if err != nil {
        return err
    }
    fmt.Printf("Found %d homes\n", len(homes.Results))
    return nil
}
```

Use `WithHTTPClient` to provide timeouts or `WithHTTPTransport` to supply a
custom `http.RoundTripper`, including a transport configured for HTTP/2. The
default client has no request timeout. `WithRegion`, `WithCloudAPIURL`, and
`WithAuthenticationURL` select endpoints; client options are validated when
`NewClient` is called. The default cloud endpoint is the US region.

HTTP transport injection also applies to the default HTTP-based RTC signaling.
Use `WithMQTTClientFactory` to replace the MQTT network client, and
`WithRTCSignaling` to replace Tuya's RTC signaling edge with a custom
implementation such as a WebSocket client. RTC media peer connections are
managed by the caller.

Token handling is explicit: `Session.Tokens` returns the current token
snapshot, `AuthService.RefreshToken` returns replacement credentials without
changing the session, and `Session.SetTokens` applies credentials to subsequent
requests. Serialize refreshes and persist the rotated access/refresh token pair
atomically in application-managed storage. Never print tokens or persist them
in source-controlled files. See the [authentication guide](docs/authentication.md).

## Errors

Operations return Go `error` values for transport, API, decoding, and protocol
failures. This package does not provide a stable exported error type for
classification, so callers should not parse error strings. Some authentication
requests carry credentials in query parameters; redact errors and request URLs
before logging them.

## Examples

- [QR-code authentication](examples/auth/README.md) reads `TUYA_ACCESS_CODE`
  from the environment and suppresses token output.
- [Read-only device queries](examples/devices/devices.go) reports counts and
  suppresses device details and local keys.
- [Device command](examples/command/command.go) sends a live command to the
  device in `TUYA_DEVICE_ID`; review the command and device before running it.
- [Token refresh](examples/token_refresh/token_refresh.go) does not print the
  replacement credentials.
- [Event listeners](examples/messaging/messages.go) suppresses broker details
  and event payloads.

The examples read credentials from environment variables. Environment
variables are convenient for local demonstrations; use an OS keychain or a
secret manager in deployed applications.

## Verification and evidence

The automated tests use `httptest` servers and hand-authored synthetic event
messages. They validate this implementation's request handling and parsing; they
are not real Tuya captures. The maintainer reports that the implemented API
flows worked with their real Tuya account(s), but those tests were not recorded
with sanitized exchanges or route-by-route provenance. The report cannot be
independently reproduced or reviewed from this repository and does not establish
behavior for every account or device model. No private captures are checked in.
`GET /v1.0/devices` is the only exact route in the checked-in provider schema;
the `/v1.0/m/...` routes are private routes without matching public wire
specifications. See [test and fixture notes](docs/testing.md) and the
[provider evidence review](docs/provider-evidence.md) for scope.

Run the checks from the module root:

| Purpose | Command |
| --- | --- |
| Build | `make build` |
| Vet | `make lint` |
| Tests with race detector | `make test` |
| Non-generated package coverage gate (80% minimum) | `make coverage` |
| All checks | `make check` |
| Format Go files | `make fmt` |

The live integration test is opt-in and requires real Tuya credentials. It
queries the account and devices; see [testing notes](docs/testing.md) before
running it.

## Documentation

- [Documentation index](docs/README.md)
- [Authentication and token handling](docs/authentication.md)
- [Device queries and commands](docs/device-control.md)
- [Message queue events](docs/events.md)
- [Tuya protocol notes](docs/TUYA.md)
- [Published API reference and customer guides](https://portpowered.github.io/go-tuya/)
- [Go API reference](https://pkg.go.dev/github.com/portpowered/go-tuya/pkg/tuya)

## License

See [LICENSE](LICENSE).
