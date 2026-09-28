# Testing and fixture provenance

## Offline checks

Run from the repository root:

```sh
make lint
make check
```

The unit tests use `httptest` and hand-authored event messages. Those examples
are synthetic and do not demonstrate verified live Tuya behavior. The
repository contains no captured provider responses or private account/device
fixtures.

The authentication example is a separate Go module because it uses a terminal
QR renderer. Compile it with:

```sh
cd examples/auth
go test ./...
```

## Opt-in live integration test

The integration test makes read-only requests for homes, devices, status, and
specifications. It is excluded from ordinary `go test ./...` runs and only
builds when the `integration` tag is supplied:

```sh
TUYA_AUTH_TOKEN="<access-token>" \
TUYA_REFRESH_TOKEN="<refresh-token>" \
TUYA_AUTH_TOKEN_EXPIRED="<expiry-milliseconds>" \
go test -tags=integration ./tuya -run TestIntegration_DeviceManagementWorkflow
```

When the tag is set but credentials are absent, the test skips. Use a test
account, keep values out of shell history where possible, and do not share
unredacted test output. The live test is not run in pull-request CI.

## Evidence limits

The package currently has no replay fixture corpus or replay coverage command.
Before describing a live response as captured evidence, sanitize it, record its
source and collection date in UTC, add a neighboring provenance note, and keep
it separate from synthetic examples. Remove tokens, cookies, personal data,
account identifiers, and device identifiers before adding any capture.
