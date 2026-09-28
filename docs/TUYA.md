# Tuya provider notes

This library implements a client for selected Tuya cloud and Device Sharing
operations. The code and offline tests describe the package surface; they do
not prove that every endpoint is available for every Tuya account, region, or
device.

## Evidence

Authentication, HTTP request handling, message parsing, and selected device
operations are exercised with `httptest` and hand-authored synthetic
responses. These are tests of the implementation contract, not captured Tuya
responses. The maintainer reports successful tests of the implemented API
flows with their real Tuya account(s), but those tests were not recorded with
sanitized exchanges or route-by-route provenance. No captured live payloads are
included in this repository.

The client uses Tuya request signing and provider-specific request formats.
Refer to the [Tuya Device Sharing SDK](https://github.com/tuya/tuya-device-sharing-sdk)
and [Tuya cloud API documentation](https://developer.tuya.com/en/docs/cloud/api-reference)
for current provider requirements. The public API may not cover all behavior
described by those sources.

The [wire-contract evidence review](provider-evidence.md) compares selected
client routes with specific official Tuya references and records gaps that
remain before publishing a provider schema.

## Token refresh

`AuthService.RefreshToken` returns replacement token values. Applications own
token storage and coordination. Serialize refresh operations and persist the
replacement pair atomically so concurrent requests do not overwrite newer
credentials with stale values. Treat tokens and request URLs that include
authentication parameters as secrets.

## Regions and unsupported surfaces

The client defaults to the US cloud endpoint. `GetRegionEndpoint` lists the
configured CN, US, EU, and India endpoints; callers can use `WithRegion` or
`WithCloudAPIURL` when constructing the reusable client. Authentication has
its own URL, configurable with `WithAuthenticationURL`. This package does not
implement local-network device control. Some declared service surfaces remain placeholders; consult the
[Go API reference](https://pkg.go.dev/github.com/portpowered/go-tuya/pkg/tuya)
and the supported-operation list in the root README before depending on them.
