# go-tuya

[![Go version](https://img.shields.io/github/go-mod/go-version/portpowered/go-tuya)](go.mod)
[![CI](https://github.com/portpowered/go-tuya/actions/workflows/go.yml/badge.svg)](https://github.com/portpowered/go-tuya/actions/workflows/go.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://portpowered.github.io/go-tuya/coverage.json)](https://portpowered.github.io/go-tuya/coverage.html)
[![Release](https://img.shields.io/github/v/release/portpowered/go-tuya?display_name=tag)](https://github.com/portpowered/go-tuya/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/portpowered/go-tuya/pkg/tuya.svg)](https://pkg.go.dev/github.com/portpowered/go-tuya/pkg/tuya)
[![License](https://img.shields.io/github/license/portpowered/go-tuya)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-guides-blue)](https://portpowered.github.io/go-tuya/)

A Go client for Tuya Device Sharing. It provides QR authentication, home and
device queries, device commands, event listeners, capability conversion, and
camera RTC signaling. Applications supply and store account credentials.

## Install

Requires Go 1.24 or later.

```sh
go get github.com/portpowered/go-tuya@latest
```

## Use an authenticated session

Create a reusable client, then a session for each account. This example assumes
the application has already loaded the credentials securely:

```go
import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/portpowered/go-tuya/pkg/tuya"
)

func listHomes(ctx context.Context, applicationClientID, accessToken, refreshToken string, expiryMilliseconds int64) error {
    base, err := tuya.NewClient(
        tuya.WithClientID(applicationClientID),
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
`Session.Tokens` returns the current credentials. Refresh explicitly with
`AuthService.RefreshToken`, persist the returned pair, then call
`Session.SetTokens`; requests do not rotate tokens automatically.

## Supported operations

- Generate and validate QR login codes, and refresh tokens.
- Query homes, devices, status, specifications, logs, factory information,
  sub-devices, and device users. Update names, submit device commands, and
  manage or remove devices.
- Listen for account and device events through the Tuya message queue.
- Request and stop camera RTC sessions when the account and device support the
  Tuya flow.
- Convert selected status codes to local capability values with
  `DeviceCapabilityMap`.

Device command codes and values depend on the device. Read its specification
before sending commands. The client does not control devices on the local
network. The API reference labels operations that are implementation-derived;
those labels describe this client's behavior, not a Tuya-verified contract.

## Configuration and lifecycle

`NewClient` accepts functional options for region or custom endpoints and for
HTTP transport configuration. `WithHTTPClient` and `WithHTTPTransport` are
mutually exclusive; the default HTTP client has no timeout. HTTP-based RTC
signaling uses the injected HTTP transport. `WithMQTTClientFactory` and
`WithRTCSignaling` replace the MQTT and RTC signaling edges. RTC media peer
connections remain application-owned.

The reusable client holds endpoint and transport settings. Each account gets a
separate `Session`, which owns its token snapshot, services, event queue, and
connection lifecycle. Call `Close` when finished. Serialize concurrent token
refreshes and store access and refresh tokens atomically in application-managed
secure storage.

HTTP, authentication, and decoding errors are `*tuya.ClientError`. Use
`errors.As` to inspect `Kind` and `errors.Is` to inspect the underlying cause.
Some authentication requests include credentials in their URLs; redact URLs
and errors before logging them.

## Guides

- [Authentication and token handling](https://portpowered.github.io/go-tuya/docs/guides/authentication)
- [Query devices](https://portpowered.github.io/go-tuya/docs/guides/device-list)
- [Devices and commands](https://portpowered.github.io/go-tuya/docs/guides/device-control)
- [Events](https://portpowered.github.io/go-tuya/docs/guides/events)
- [Standalone CLI](https://portpowered.github.io/go-tuya/docs/guides/cli)
- [Generated API reference](https://portpowered.github.io/go-tuya/)

The standalone CLI is maintained as a separate module under `cmd/go-tuya`.
Its first published release is pending; the guide documents checkout usage
and installation after a release is available.

## License

See [LICENSE](LICENSE).
