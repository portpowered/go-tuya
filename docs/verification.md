# Verification and evidence

This contributor note records the checks and evidence boundaries for the Tuya
client. The complete schema-to-code and call-site population is generated in
[wire-model-inventory.md](wire-model-inventory.md). Provider documentation
links and implementation-derived labels are attached to individual schemas
and operations.

## Local checks

Run the blocking checks from the repository root:

```sh
make lint
make check
make coverage
make replay
```

`make check` runs lint, generated-schema and route checks, builds, and
race-enabled tests for the SDK, CLI, and authentication example. `make
coverage` reports non-generated coverage for the public SDK, transport
packages, and their combined total; the combined minimum is 80%, with a 90%
target. Generated files are excluded. Review the remaining uncovered
production paths rather than adding tests only to raise the percentage.

## Paired synthetic replays

The checked-in HTTP fixtures are synthetic. The paired replay transport
matches method, origin, escaped path, repeated query values, relevant headers,
and body before returning the matching response. It rejects unexpected and
duplicate calls and checks ordered exchanges where order matters. The
authentication and encrypted routes use explicit match rules for volatile
values and redact credentials.

The MQTT success and denied-CONNACK tests exercise the pinned Paho
`v1.5.1` client through `SetCustomOpenConnectionFn` and `net.Pipe`. They
match MQTT packet bytes, check the connection lifecycle, and require EOF after
disconnect or failed connection. The external dependency contract is in
[`api/external/paho-mqtt-v1.5.1.yaml`](../api/external/paho-mqtt-v1.5.1.yaml).

No sanitized Tuya account captures are checked in. Synthetic replays establish
the current implementation's behavior; they do not prove provider behavior
for every account, region, device, or event.

## Release verification

Release workflows must verify the exact SDK and nested CLI tag commits through
the Go module proxy and compile them in clean consumer modules. The CLI's local
`replace` directive is for development and must be absent from a published
CLI module. Do not treat a passing pull-request CI run or an earlier tag as
proof for a later release commit. See [releasing.md](releasing.md) for the
release sequence.
