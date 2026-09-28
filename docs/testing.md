# Testing and fixture provenance

## Offline checks

Run the library checks from the repository root:

```sh
make lint
make check
make coverage
```

`make coverage` measures statement coverage for non-generated Go files under
`pkg/` and fails below 80%. New synthetic cases should bring the public
library toward 90% without presenting synthetic data as provider captures.
The same 80% floor runs in CI.

Run the replay suite separately when measuring offline API replay coverage:

```sh
make replay
```

The replay tests inject an `http.RoundTripper` and serve response bodies from
`tests/replay/fixtures`. They cover QR login and validation, then an authenticated
read-only home, device-list, and device-status flow, a paginated global device
list, and an API error.
Every response file is marked `.synthetic.json` and has a neighboring
`PROVENANCE.md`. These are hand-authored examples for testing the current client;
they are not Tuya captures and do not verify private vendor routes or response
schemas. `make replay` reports coverage for `tuya` code reached by the replay
suite. The library coverage gate and replay coverage are separate measurements.

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
go test -tags=integration ./pkg/tuya -run TestIntegration_DeviceManagementWorkflow
```

When the tag is set but credentials are absent, the test skips. Use a test
account, keep values out of shell history where possible, and do not share
unredacted test output. The live test is not run in pull-request CI.

## Evidence limits

The maintainer reports successful tests of the implemented API flows with their
real Tuya account(s). Those tests were not documented with sanitized captures or
route-by-route provenance. The repository therefore has no captured provider
responses or private account/device fixtures, and the report cannot be replayed
or independently inspected here. Before describing a live response as captured
evidence, sanitize it, record its source and collection date in UTC in a
neighboring provenance note, and keep it separate from synthetic examples.
Remove tokens, cookies, personal data, account identifiers, and device
identifiers before adding any capture.
