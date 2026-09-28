# Device-sharing fixture provenance

These paired request and response exchanges are hand-authored synthetic
examples for the current client's home, device-list, device-status, and
API-error handling. They were
not captured from Tuya or a Tuya account. `global-device-list.synthetic.json`
uses the documented `GET /v1.0/devices` logical result shape to check the
generated-model adapter. Its outer `success`/`t` envelope is synthetic and
matches the current parser; Tuya's route example does not specify that wrapper.
The fixture does not verify the client's encrypted transport.
Home, device, product, user, and transaction IDs are fabricated; local-key
values are synthetic and no real credential is included. The icon examples
use the reserved `example.invalid` domain, and example IP addresses are from a
documentation-only network range. Other fixture shapes are test inputs only
and do not verify private Tuya routes or their wire contracts.
