# Device-sharing fixture provenance

These paired request and response exchanges are hand-authored synthetic
examples for all 28 HTTP schema operations, plus an API-error outcome. The
remaining operation pairs are generated from the checked-in synthetic inputs
in `make_remaining.py`; the replay test validates the resulting stored JSON.
They were
not captured from Tuya or a Tuya account. `global-device-list.synthetic.json`
uses the documented `GET /v1.0/devices` logical result shape to check the
generated-model adapter. Its outer `success`/`t` envelope is synthetic and
matches the current parser; Tuya's route example does not specify that wrapper.
The replay harness verifies encrypted request query/body meanings and HMAC
signatures. The string result in `addDeviceUser` is encrypted with a fixed
synthetic nonce and the runtime request ID, since the provider response key
depends on that ID. This is a fixture-defined transform, not a response
fallback. These checks do not verify Tuya's live transport.
Home, device, product, user, and transaction IDs are fabricated; local-key
values are synthetic and no real credential is included. The icon examples
use the reserved `example.invalid` domain, and example IP addresses are from a
documentation-only network range. Other fixture shapes are test inputs only
and do not verify private Tuya routes or their wire contracts.
