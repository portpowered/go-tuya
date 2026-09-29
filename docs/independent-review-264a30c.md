# Independent standards review at `264a30c`

Reviewer: independent Codex reviewer; I did not implement this commit or the
Tuya migration. Reviewed implementation commit:
`264a30c9669c0a25d8a73a2968ec37b52225f663` (`ci: enforce all golangci-lint
checks`, 2026-09-29). The source standards are the [library
standards](https://github.com/portpowered/go-third-party-template/blob/main/docs/library-standards.md)
and its linked [verification](https://github.com/portpowered/go-third-party-template/blob/main/docs/verification.md),
[client design](https://github.com/portpowered/go-third-party-template/blob/main/docs/client-design.md),
and [website](https://github.com/portpowered/go-third-party-template/blob/main/docs/website.md)
guides.

The review covers the current implementation, tests, lint configuration,
workflows, replay fixtures, API schemas, README and customer guides. I checked
the current source and evidence rather than carrying forward the v0.3.3
sign-off. No implementation finding remains open.

## Itemized verdicts

| Item | Verdict | Evidence at the reviewed commit |
| --- | --- | --- |
| 1. Standalone library | Verified | `pkg/tuya`, root examples, `README.md`, and `docs/guides/` are provider-facing. The repository contains no consuming-application adapter or rollout plan. |
| 2. Operations, errors, and transports | Verified | README sections “Supported surface,” “Authentication and requests,” and “Errors,” plus nine MDX guides, describe the exported API. Examples call public `tuya` methods. Synthetic and implementation-derived behavior is distinguished from undocumented account testing. `ClientError` exposes stable kinds and unwraps its cause. |
| 3. Repository badges | Verified | `README.md` lines 3–9 show all seven required badges for Go version, CI, filtered coverage, release, Go Reference, license, and docs, each pointing to this repository/module. |
| 4. Schema-backed wire inventory | Verified | OpenAPI contains all 28 HTTP operations and generated method/path descriptors; AsyncAPI contains all three topic templates. The current Documentation run regenerated models/routes and passed its checked-in-artifact comparison. See the inventory and negative tests below. |
| 5. Blocking all-linter gate and offline checks | Verified | `.golangci.yml` has literal `linters.default: all`; CI pins v2.14.0, runs the whole repository, accepts all issues rather than only new issues, and has no bypass. Exact-sha CI run [36546527523](https://github.com/portpowered/go-tuya/actions/runs/36546527523) passed its “Lint (all linters)” step with 0 issues and all subsequent gates. I audited every config exclusion and source annotation below. |
| 6. Synthetic coverage | Verified | Exact-sha CI reports 82.8% non-generated coverage (1,336/1,614 statements) against the 80% floor. `tools/coverage` excludes generated files and has an explicit path-escape test. Remaining low-coverage branches and the 90% stretch target are described in `docs/guides/testing.mdx`. |
| 7. Package boundaries and consumer | Verified | Public package is `github.com/portpowered/go-tuya/pkg/tuya`; generated private wire code is under `pkg/tuya/internal/wire`. I created a fresh temporary consumer module importing the exact working-tree module, ran `go mod tidy` and `go test ./...`, and both passed. |
| 8. Option-based client construction | Verified | `NewClient(...Option)` validates endpoints, client ID and transports; `pkg/tuya/client_test.go` covers defaults, invalid options, conflicting HTTP options, region selection and injected transports. |
| 9. Stateless client and explicit session | Verified | `Client.NewSession` creates account-local token and queue state. Tests cover independent sessions, queue connection errors, status, stop/cancellation and callback backpressure under race detection. |
| 10. Injectable network edges | Verified | HTTP uses `WithHTTPClient`/`WithHTTPTransport`, MQTT uses `WithMQTTClientFactory`, and RTC can use `WithRTCSignaling`. Synthetic tests exercise those seams, including the real queue `Connect` path in the paired MQTT transcript. There is no built-in WebSocket route. |
| 11. Caller-managed credentials | Verified | `Session.Tokens` and `SetTokens` expose caller-owned state. Refresh returns rotated values without mutating the session; `TestAuthService_RefreshTokenReturnsRotatedTokensWithoutChangingSession` verifies that behavior. |
| 12. Published MDX guides | Verified | All nine customer guides are under `docs/guides/*.mdx`. Exact-sha Documentation run [36546527519](https://github.com/portpowered/go-tuya/actions/runs/36546527519) regenerated schema references, built Fumadocs, passed the rendered-site link check, uploaded the Pages artifact and deployed it. The live guide index and testing/provider-evidence pages returned HTTP 200. |
| 13. Published copy | Verified | I reviewed the current customer-facing README and guide sources, including the only customer-guide copy change since the v0.3.3 page review (`docs/guides/testing.mdx`). The exact-sha build/link/deploy passed; schema and generated reference copy did not change. Provider evidence remains qualified, and release-note links still point to the published evidence guide and versioned changelog. |
| 14. Independent verification | Verified | This review checks every numbered item at the exact implementation SHA and records findings/dispositions here. I inspected the eight restored package test files listed under “Test and artifact audit,” plus the remaining changed test files. No test or example function was removed; added tests cover malformed body values, mixed-case headers and coverage path traversal. |
| 15. Paired replay | Verified | All 28 HTTP operation IDs have paired synthetic requests and responses. HTTP checks validate method, origin, escaped path, query values, relevant headers, encrypted meaning and signature before returning the paired response. The ordered MQTT transcript covers paired configuration HTTP, client creation/connect, subscriptions, event callbacks, unsubscribe and disconnect. Mismatch, duplicate, volatile-value and exhaustion cases pass in exact-sha CI. |

**Disposition:** all 15 items pass at `264a30c9669c0a25d8a73a2968ec37b52225f663`.
There are no unresolved findings. The docs-only review commit and checklist
update are follow-up evidence; the implementation SHA above remains the code
under review.

## Item 4: current HTTP and MQTT inventory

Each HTTP operation below is present in `api/openapi.yaml`, has a generated
`wire.Operation<Name>()` descriptor in
`pkg/tuya/internal/wire/routes.gen.go`, and is dispatched through the named
client source file. Private `/v1.0/m/...` and `/v1.1/m/...` entries are
implementation-derived, not claimed as public Tuya specifications.

| Generated operation | Method and schema path | Client call site |
| --- | --- | --- |
| AddDeviceUser | `POST /v1.0/devices/{device_id}/user` | `pkg/tuya/devices.go` |
| DeleteDevice | `DELETE /v1.0/devices/{device_id}` | `pkg/tuya/devices.go` |
| DeleteDeviceUser | `DELETE /v1.0/devices/{device_id}/users/{user_id}` | `pkg/tuya/devices.go` |
| GenerateLoginQRCode | `POST /v1.0/m/life/home-assistant/qrcode/tokens` | `pkg/tuya/auth.go` |
| GetDeviceDetails | `GET /v1.0/devices/{device_id}` | `pkg/tuya/devices.go` |
| GetDeviceList | `GET /v1.0/devices` | `pkg/tuya/devices.go` |
| GetDeviceLogs | `GET /v1.0/devices/{device_id}/logs` | `pkg/tuya/devices.go` |
| GetDeviceUser | `GET /v1.0/devices/{device_id}/users/{user_id}` | `pkg/tuya/devices.go` |
| GetDevicesByUser | `GET /v1.0/users/{uid}/devices` | `pkg/tuya/devices.go` |
| GetFactoryInfos | `GET /v1.0/devices/factory-infos` | `pkg/tuya/devices.go` |
| GetMessageQueueConfig | `POST /v1.0/m/life/ha/access/config` | `pkg/tuya/messages.go` |
| GetSubDevices | `GET /v1.0/devices/{device_id}/sub-devices` | `pkg/tuya/devices.go` |
| ListDeviceUsers | `GET /v1.0/devices/{device_id}/users` | `pkg/tuya/devices.go` |
| ListMultiOutletNames | `GET /v1.0/devices/{device_id}/multiple-names` | `pkg/tuya/devices.go` |
| QueryDeviceSpecification | `GET /v1.1/m/life/{device_id}/specifications` | `pkg/tuya/devices.go` |
| QueryDeviceStatus | `GET /v1.0/m/life/devices/{device_id}/status` | `pkg/tuya/devices.go` |
| QueryHomeDevices | `GET /v1.0/m/life/ha/home/devices` | `pkg/tuya/devices.go` |
| QueryHomes | `GET /v1.0/m/life/users/homes` | `pkg/tuya/homes.go` |
| RefreshAccessToken | `GET /v1.0/m/token/{refresh_token}` | `pkg/tuya/auth.go` |
| ResetDeviceFactory | `PUT /v1.0/devices/{device_id}/reset-factory` | `pkg/tuya/devices.go` |
| SendDeviceCommands | `POST /v1.1/m/thing/{device_id}/commands` | `pkg/tuya/devices.go` |
| StartRTCSession | `POST /v1.0/m/life/ipc/{device_id}/webrtc/session` | `pkg/tuya/client_rtc.go` |
| StopRTCSession | `DELETE /v1.0/m/life/ipc/{device_id}/webrtc/session/{session_id}` | `pkg/tuya/client_rtc.go` |
| UpdateDeviceFunctionName | `PUT /v1.0/devices/{device_id}/functions/{function_code}` | `pkg/tuya/devices.go` |
| UpdateDeviceName | `PUT /v1.0/devices/{device_id}` | `pkg/tuya/devices.go` |
| UpdateDeviceUser | `PUT /v1.0/devices/{device_id}/users/{user_id}` | `pkg/tuya/devices.go` |
| UpdateMultiOutletName | `PUT /v1.0/devices/{device_id}/multiple-name` | `pkg/tuya/devices.go` |
| ValidateLoginCode | `GET /v1.0/m/life/home-assistant/qrcode/tokens/{login_code}` | `pkg/tuya/auth.go` |

The checked-in AsyncAPI schema and generated `wire.MQTTChannel...` templates
define `ownerEvents` (`{ownerTopic}`), `deviceStatus` (`{deviceTopic}/sta`),
and `deviceLocal` (`{deviceTopic}/pen`). The queue subscribes to owner events
and device status; device local is currently formatted for topic addressing
but is not subscribed. HTTP call sites use generated operation descriptors or
generated direct-auth method/path constants. MQTT subscription lifecycle
wrappers use generated templates. Generated wire models also cover encrypted
HTTP envelopes, query/body fields, MQTT 4/20 envelopes, nested event fields,
and message-queue configuration.

Negative checks in `tools/wireroutes/main_test.go` reject an unschematized path,
a changed method that retains the path, an unknown channel, direct MQTT
subscribe/unsubscribe calls, and unregistered operation call sites. The
exported raw encrypted request path also rejects an unknown method/path pair.
The exact-sha CI route gate passed. The exact-sha Documentation run passed
generation of models and routes plus `git diff --exit-code` checks for all
three tracked generated files.

## Item 5: blocking lint gate and exception audit

The exact-sha main CI workflow has one `verify` job with no conditional skip
or `continue-on-error`. Its lint step uses `golangci/golangci-lint-action@v9`
with `version: v2.14.0`, `only-new-issues: false`, and only a timeout argument.
The exact run log records the invoked command as
`golangci-lint run --timeout=5m` from the repository root and reports `0
issues.` There is no `--issues-exit-code=0`, issue filter, or later step that
can mask a lint failure. Release CI uses the same pinned linter and full-repo
blocking behavior. The Makefile pins v2.14.0 by default and `make lint` passes
`./...`. `make check`, `make replay`, and `make coverage` also passed locally;
the main CI run passed build, schema gate, race tests, coverage, replay, vet,
formatting and module metadata.

The config has the required literal `linters.default: all` and
`generated: strict`; there is no disabled-linter list. Every
`linters.exclusions.rules` entry is scoped by both path and diagnostic text,
names its linter, and has a reason immediately above it:

| Linter exception | Scope | Stated reason |
| --- | --- | --- |
| `exhaustruct`, `exhaustruct_v5` | “is missing fields?” only in 12 focused test files: `pkg/tuya/{auth,capabilities,client_rtc,client,encryption,message_protocol,messages_synthetic,mqtt_replay,route_synthetic}_test.go`, `tests/replay/{complete_replay,replay}_test.go`, `tools/compatibility/main_test.go` | Fixtures intentionally populate only fields relevant to the case. |
| `noinlineerr` | “avoid inline error handling” only in 10 test files: `pkg/tuya/{client,encryption,message_protocol,messages_synthetic,mqtt_replay,route_synthetic}_test.go`, `tests/replay/{complete_replay,replay}_test.go`, and `tools/{compatibility,coverage}/main_test.go` | Short-scoped errors keep assertions independent in tests. |
| `testpackage` | Exact package-name diagnostic only in 9 `pkg/tuya` tests: `auth`, `capabilities`, `client_rtc`, `client`, `encryption`, `message_protocol`, `messages_synthetic`, `mqtt_replay`, and `route_synthetic` test files | These tests inspect the named package-private state, conversion logic, transports or fixtures. |
| `unparam` | Exact `sid always receives ""` diagnostic in `pkg/tuya/encryption.go` | The parameter remains for provider authorization context; current calls use empty SID. |
| `forbidigo` | `fmt.Print` diagnostics only in `examples/{command/command,devices/devices,messaging/messages,token_refresh/token_refresh}.go` and `tools/{compatibility,coverage}/main.go` | These commands intentionally print their user-facing output. |

I also inspected every source `//nolint` and `// #nosec` annotation returned
by a repository-wide search. Each names one or more rules and has a local
reason. Field-level `tagliatelle` exceptions in `pkg/tuya/api.go`,
`capabilities.go`, `message_protocol.go`, and `messages.go` preserve schema or
provider-defined camelCase JSON keys. `interfacebloat` is limited to the
existing public aggregate in `api.go`; `nilnil` documents an absent optional
parameter; `ireturn` annotations explain generic result or required Paho
interface boundaries. Complexity exceptions are function-local, primarily
for complete table-driven conversion fixtures and replay/lifecycle assertions;
the one production `funcorder,funlen` exception keeps a category map beside
its private constructors. Security annotations name individual gosec rules at
the specific fixed, temporary, caller-selected or validated file/process
operation and explain the input boundary. No unreasoned or broad file-level
source suppression was found. CI emitted deprecation warnings for legacy
linter aliases but still ran the configured set, returned zero findings, and
passed; the warnings do not bypass the gate.

## Test and artifact audit

The eight restored package test files were
`pkg/tuya/auth_test.go`, `capabilities_test.go`, `client_rtc_test.go`,
`client_test.go`, `encryption_test.go`, `message_protocol_test.go`,
`messages_synthetic_test.go`, and `mqtt_replay_test.go`. All prior test and
example function names in these files remain at this commit; the new
function-level coverage adds cases rather than replacing a test. The comments
removed or moved in the diffs were punctuation fixes, section headings, or
labels carried alongside the refactored test/helper; request and mapping
assertions remain. The route, replay and coverage test files were also
reviewed. `tests/replay/test_errors_test.go` is present in the commit tree and
provides the synthetic sentinels used by the paired replay tests.

The only JSON-tag value changed in the implementation diff is the test-only
`Count` field in `TestWireStringMapRejectsNonStringFields`, from
`json:"X-count"` to `json:"x_count"`. That test asserts that an integer header
value is rejected; it does not assert an emitted wire key. Production and
generated JSON tags did not change. Header spelling and wire behavior remain
covered independently by `TestEncryptedClient_MakeRequestPayload`, which
checks `X-Appkey`, signed header values and the recomputed signature, and by
`TestMatchFixtureHeadersUsesCanonicalCaseMatching`, which matches actual
`X-Appkey` against the paired fixture key `X-appKey`. The replay matcher uses
`http.Header.Values(name)`, so canonical case matching is exercised directly.
This tag cleanup does not weaken a wire-key replay assertion.

`ClientError.Unwrap` returns the original `Cause`; the synthetic transport
test checks both `errors.As` classification and `errors.Is(err, cause)`.
`coverageSource` normalizes separators, cleans the relative path and rejects
absolute paths or `..` escapes before reading package sources;
`TestReportRejectsCoveragePathOutsidePackageRoot` verifies the escape is
rejected with the sentinel error. The committed source tree contains no
`captured/` fixtures, credentials, private account captures, `.env` files, or
generated executables. Replay examples live under the `synthetic` fixture
directories with provenance notes. The local pre-existing untracked coverage
and `.codex-tmp` artifacts were not added to this review commit.

## Exact-run evidence

- Main CI [36546527523](https://github.com/portpowered/go-tuya/actions/runs/36546527523),
  head SHA `264a30c9669c0a25d8a73a2968ec37b52225f663`: success. The blocking
  all-linter step and all build, schema, race, filtered-coverage, replay, vet,
  formatting and metadata steps passed.
- Documentation [36546527519](https://github.com/portpowered/go-tuya/actions/runs/36546527519),
  same head SHA: success. Schema model/route generation, generated-file drift,
  site build, rendered-site links, coverage artifact and Pages deployment
  passed.
- Independent local checks already recorded for this exact tree: `make lint`,
  `make check`, `make replay`, `make coverage`, and `make generate-wire` plus
  generated-file diff verification passed. The separate external consumer
  module check passed as described under item 7.
