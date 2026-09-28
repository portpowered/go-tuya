# Independent standards verification: go-tuya

## Full 15-item re-review at `v0.3.3`

Reviewer: independent Codex reviewer, not an implementer of the Tuya code.
The reviewed tag `v0.3.3` resolves to
`2b598febb4f656e407e0f6a525c8633063def27b`. Relative to the prior
item-15 review commit `0364688`, its only change formats
`pkg/tuya/mqtt_replay_test.go`; no behavior, schema, fixture, or site source
changed. The previous `v0.3.2` tag at `0364688` passed Release
[`36495543940`](https://github.com/portpowered/go-tuya/actions/runs/36495543940)
and Documentation but failed main CI
[`36495455240`](https://github.com/portpowered/go-tuya/actions/runs/36495455240)
because that test was not gofmt-formatted. At `2b598fe`, main CI
[`36495828460`](https://github.com/portpowered/go-tuya/actions/runs/36495828460),
Documentation [`36495828389`](https://github.com/portpowered/go-tuya/actions/runs/36495828389),
and exact-tag Release [`36495952279`](https://github.com/portpowered/go-tuya/actions/runs/36495952279)
passed. The release verify job passed generated-wire drift, coverage,
compatibility, build, race, vet, paired replay, and a separate consumer
fetching `v0.3.3` through the public Go proxy. I reran `make lint`, `make
check`, and `make replay` locally; all passed, and `gofmt -l` is empty.

| Standard | Final-commit verdict | Independent evidence |
| --- | --- | --- |
| 1. Standalone | Verified | Package, examples, README, and site are provider-focused; no consuming-application adapter was added since the prior inventory below. |
| 2. Operations and errors | Verified | Exported API examples, authentication, operation, error, transport, and provider-evidence guides remain consistent. Synthetic replay is labeled separately from undocumented maintainer account testing. |
| 3. Badges | Verified | All seven README badges still point to this repository's live reports or package documentation. The `v0.3.3` versioned Go Reference page returned HTTP 200 and contains the `pkg/tuya` API. |
| 4. Generated wire surface | Verified | The 28 HTTP operations, three MQTT channel templates, generated models/descriptors, call-site inventory, and negative route/channel tests are detailed below. Exact-tag generation/drift and inventory gates passed again in `v0.3.3` Release. |
| 5. Offline checks and provenance | Verified | Local lint, build/race/check, replay, and exact-tag checks passed. HTTP and MQTT pairs are explicitly synthetic; no private captures were introduced. |
| 6. Coverage | Verified | Exact-tag coverage gate passed the 80% non-generated floor; the prior direct measurement was 82.6% (1,308/1,583). Generated exclusions and low-coverage branches remain documented. |
| 7. Package boundaries | Verified | Public import is `pkg/tuya`, private generated wire is `pkg/tuya/internal/wire`, and the exact-tag public-proxy job compiled a separate `v0.3.3` consumer. |
| 8. Options | Verified | `NewClient(...Option)` and tests cover endpoint, HTTP, MQTT, RTC, region, client ID, validation, and conflicting HTTP options. No account token is reusable client configuration. |
| 9. Session state | Verified | `Session` owns tokens and queue/RTC lifecycle; `MessageQueue` exposes initial and later errors, deterministic stop, and serial callback pressure. Race-enabled lifecycle tests passed. |
| 10. Injectable network edges | Verified | HTTP transport/client, MQTT factory, and RTC signaling seams are exercised, including the actual MQTT `Connect` path in paired replay. |
| 11. Explicit credentials | Verified | Refresh returns tokens without mutating session state; callers read/set the token snapshot explicitly. Synthetic tests cover rotation. |
| 12. MDX guides | Verified | Nine customer guides remain under `docs/guides/*.mdx` and link generated HTTP/MQTT references. The final-commit Documentation build and rendered-site link checker passed. |
| 13. Concise published copy | Verified | Prior 46-page rendered copy audit below remains applicable because site sources did not change. The published `v0.3.3` notes link the live provider-evidence guide and `v0.3.2...v0.3.3` changelog, with the undocumented-account caveat. |
| 14. Independent verification | **Verified** | I independently inspected all 15 items at the final tag, reran affected checks, verified CI/Documentation/Release and the public consumer, and checked the published release, Pages guide, and versioned Go Reference. Earlier format and MQTT replay findings were corrected and rechecked. No finding remains open. |
| 15. Paired replay | Verified | All 28 HTTP operations have consumed request/response pairs; the MQTT transcript runs real queue startup through paired HTTP config, MQTT factory/`Connect`, subscriptions, events, callbacks, unsubscribe, and disconnect. Strict negative and exhaustion tests passed again at the formatted tag. See [paired-replay review](paired-replay-review.md). |

The [published `v0.3.3` release](https://github.com/portpowered/go-tuya/releases/tag/v0.3.3),
[provider-evidence guide](https://portpowered.github.io/go-tuya/docs/guides/provider-evidence/),
[versioned Go Reference](https://pkg.go.dev/github.com/portpowered/go-tuya@v0.3.3/pkg/tuya),
and public proxy version endpoint returned HTTP 200. The Go Reference page
contains `NewClient` and `MessageQueue` from the exported `pkg/tuya` API.
**All 15 items are verified at `2b598fe`; item 14 is signed off.** The
commit that adds this final report and checklist signoff is documentation-only
relative to the tag and is recorded in Git history.

## Renewed review at `efde7aef0988ef3a068cf383aaca29e318bece0c`

The prior itemized table below is the historical 14-item review for
`v0.3.1`. The shared template now has item 15. An independent reviewer
rechecked each finding from the [paired-replay review](paired-replay-review.md)
against the new synthetic HTTP and MQTT transcripts. **Item 15 is verified:**
all 28 OpenAPI operations have stored request/response pairs; the HTTP
matchers validate encrypted meaning and HMAC, and reject mismatches,
duplicates and unconsumed requests. MQTT replay now executes actual queue
startup through the paired config HTTP request, injected factory,
`mqtt.Client.Connect`, subscription, message callbacks, unsubscribe and
disconnect. Fresh targeted race tests, `make replay`, `make lint`, `make
check`, and `make coverage` passed at `efde7ae`; coverage was 82.6%
(1,308/1,583 non-generated statements).

Items 1–13 retain the prior source verdicts below because the changes since
the reviewed release affect tests, fixtures, and their guidance. The new
fixture provenance is synthetic, with no account-capture claim. **Item 14 is
open:** `v0.3.1` passed its exact-tag gates before the new item-15 suite
existed. A new final tag must run generation/drift, route/channel, coverage,
replay, race, compatibility, public-consumer and documentation gates, and the
reviewer must recheck all 15 items at that final commit before renewing the
independent signoff.

Reviewer: independent Codex reviewer (not an implementer of the Tuya migration)

Reviewed implementation commit: `1123ac3088a8b535267997330d7187f565cc162b` (2026-09-28).
The provisional report commit was `e30b71238388f8093acb555dd3a2557900078ee4`.
The successful `v0.3.1` release tag points to
`37584e21efc67b50ddfabea459c7b3cf428bab64`. The only change between the
provisional report and tag was selecting Go 1.25 in the release workflow. The
commit containing this final report and checklist signoff is documentation-only
relative to the tag; its SHA is recorded in Git history.

The criteria are the 14 numbered items in
[`go-third-party-template/docs/library-standards.md`](https://github.com/portpowered/go-third-party-template/blob/main/docs/library-standards.md),
plus its linked verification, client-design, website, and release guides.
Implementation-derived schema entries are evidence of this client's behavior,
not a provider-verified Tuya contract. The maintainer reports account testing,
but the exchanges were not recorded with sanitized, operation-level provenance.

## Itemized verdicts

| Item | Verdict | Independent evidence and remaining work |
| --- | --- | --- |
| 1. Standalone library | Verified | `pkg/tuya`, root examples, README, and `docs/guides` describe provider use; a search found no portos-backend import, application adapter, or rollout plan. The Home Assistant wording names Tuya's private route family, not a consuming application dependency. |
| 2. Operations and errors | Verified | README and MDX guides describe supported authentication, devices, commands, events, transport options, and synthetic versus undocumented account evidence. Examples use exported methods. `ClientError.Kind` provides stable invalid-operation, unauthorized, not-found, transport, provider, and protocol classes; `Unwrap` retains causes. README and authentication MDX document `errors.As`; synthetic tests cover transport, provider HTTP statuses, and replayed API errors. |
| 3. Badges | Verified | README has Go version, CI, coverage, release, Go Reference, license, and Documentation badges with this repository's destinations. `v0.3.1` is published; its versioned Go Reference page and the Pages coverage report return HTTP 200. |
| 4. Complete schema-generated wire surface | Verified on `v0.3.1` | See the operation/channel inventory and negative tests below. OpenAPI and AsyncAPI, generated descriptors, parameter and header models, encrypted envelopes, nested MQTT payload models, and call sites were inspected. The exported raw encrypted methods reject method/path pairs outside the generated inventory; unused handwritten raw/legacy response models were removed. Exact-tag Release run `36489207604` passed regeneration/drift, method/path/channel gate, and 82.2% non-generated coverage before publication. |
| 5. Offline checks and fixture provenance | Verified locally | `make lint`, `make check` (build, route gate, race tests), and `make replay` passed at the reviewed commit. `tests/replay/fixtures/**/synthetic` has provenance notes; no captured fixtures are claimed. Current live account tests remain undocumented. |
| 6. Synthetic coverage | Verified | `make coverage` measured 82.2% (1,302/1,583) non-generated statements, above the CI 80% floor; `tools/coverage` excludes `*.gen.go`/generated markers and reports package and combined values. Synthetic tests exercise queue failure, reconnect, status, stop, backpressure, and typed error paths. `docs/guides/testing.mdx` and the checklist report remaining low-coverage MQTT retry/expiry branches and unimplemented placeholders. The 90% target is not a hard gate. |
| 7. Package boundaries | Verified on `v0.3.1` | Public package is `pkg/tuya`; schema-generated private models/descriptors are `pkg/tuya/internal/wire`; examples and replay fixtures are separate. The exact-tag release job fetched `v0.3.1` through the public Go proxy and compiled a fresh separate consumer of `pkg/tuya`; versioned Go Reference returned HTTP 200. |
| 8. Option-based construction | Verified | `NewClient(...Option)`, endpoint validation, HTTP, MQTT, RTC, region, and client-ID options exist and have synthetic tests. `WithHTTPClient` and `WithHTTPTransport` reject duplicates and mutual conflicts in either order; a synthetic test covers both orders. Account tokens remain outside reusable client options. |
| 9. Session state and lifecycle | Verified | `Client` holds reusable options, while `Session` owns tokens, queue, and RTC state. `MessageQueue.Status` exposes connection state/error; `Start` reports initial failure and `Stop` cancels and waits for the loop. Callbacks run synchronously and serially, applying backpressure rather than spawning unbounded work; the events guide states that policy. Deterministic race-enabled tests exercise connection failure, reconnect, stop, and blocked-callback delivery. |
| 10. Transport injection | Verified | `WithHTTPClient`/`WithHTTPTransport` cover auth, encrypted cloud HTTP and built-in HTTP RTC signaling; `WithMQTTClientFactory` and `WithRTCSignaling` cover MQTT and alternate signaling edges. Synthetic HTTP, MQTT, and RTC tests use those seams. No built-in WebSocket endpoint exists. |
| 11. Explicit credentials | Verified | `AuthService.RefreshToken` returns rotated credentials without mutating the session; `Session.Tokens` and `SetTokens` expose caller-managed state. Synthetic refresh tests check the unchanged session until explicit assignment. |
| 12. MDX customer guides | Verified at the reviewed commit | Nine `docs/guides/*.mdx` pages cover important workflows and link generated OpenAPI/AsyncAPI references. QR example instructions moved from `examples/auth/README.md` into authentication MDX; its README is now a pointer. Documentation deployed for `1123ac3`; I downloaded its 46-page artifact and reran the full link checker (773 internal links passed). |
| 13. Concise published copy | Verified | I inspected the rendered root, guide, 28 OpenAPI, and three AsyncAPI pages. All 28 operation descriptions label generated cURL as a route illustration and point to the signed/encrypted client flow; global cloud servers and QR-specific auth servers match client defaults. The 46-page link checker found 773 valid internal links, and all 17 external destinations returned HTTP 200. Fumadocs hydrates server selection from schema; a coordinator observed the correct cloud/QR hosts in the live browser. Its static prerender contains an `example.com` fallback, so that text is not treated as a live snippet. The corrected `v0.3.1` release notes summarize the changes and link the Pages evidence guide and `v0.2.0...v0.3.1` changelog. |
| 14. Independent signoff | Verified | This reviewer did not implement the migration. Each item above was inspected against the code, generated artifacts, call sites, tests, CI, docs, and release record. The exact-tag run passed; all recorded findings were corrected and rechecked. The final report/checklist commit changes documentation only relative to tag `37584e2`; its CI and Documentation status should be checked after push as a final publication confirmation. |

## Item 4: HTTP operation inventory

Every row below has a checked-in OpenAPI path/method and generated
`wire.Operation<Name>()` descriptor in `pkg/tuya/internal/wire/routes.gen.go`.
The named file contains the corresponding client call site. Private `/v1.0/m`
and `/v1.1/m` behavior is labeled implementation-derived in the schema.
Schema-generated parameter models, bodies, response envelopes, and transport
headers are in `models.gen.go`; the client maps these to separate public result
types. The shared encrypted envelope is `wire.EncryptedDataEnvelope` and
`wire.EncryptedHTTPResponseEnvelope`. The two direct login calls use generated
methods/routes and `wire.GenerateLoginQRCodeParams`/
`wire.ValidateLoginCodeParams`.

| Generated operation | Method and schema path | Client call site |
| --- | --- | --- |
| AddDeviceUser | `POST /v1.0/devices/{device_id}/user` | `devices.go` |
| DeleteDevice | `DELETE /v1.0/devices/{device_id}` | `devices.go` |
| DeleteDeviceUser | `DELETE /v1.0/devices/{device_id}/users/{user_id}` | `devices.go` |
| GenerateLoginQRCode | `POST /v1.0/m/life/home-assistant/qrcode/tokens` | `auth.go` |
| GetDeviceDetails | `GET /v1.0/devices/{device_id}` | `devices.go` |
| GetDeviceList | `GET /v1.0/devices` | `devices.go` |
| GetDeviceLogs | `GET /v1.0/devices/{device_id}/logs` | `devices.go` |
| GetDeviceUser | `GET /v1.0/devices/{device_id}/users/{user_id}` | `devices.go` |
| GetDevicesByUser | `GET /v1.0/users/{uid}/devices` | `devices.go` |
| GetFactoryInfos | `GET /v1.0/devices/factory-infos` | `devices.go` |
| GetMessageQueueConfig | `POST /v1.0/m/life/ha/access/config` | `messages.go` |
| GetSubDevices | `GET /v1.0/devices/{device_id}/sub-devices` | `devices.go` |
| ListDeviceUsers | `GET /v1.0/devices/{device_id}/users` | `devices.go` |
| ListMultiOutletNames | `GET /v1.0/devices/{device_id}/multiple-names` | `devices.go` |
| QueryDeviceSpecification | `GET /v1.1/m/life/{device_id}/specifications` | `devices.go` |
| QueryDeviceStatus | `GET /v1.0/m/life/devices/{device_id}/status` | `devices.go` |
| QueryHomeDevices | `GET /v1.0/m/life/ha/home/devices` | `devices.go` |
| QueryHomes | `GET /v1.0/m/life/users/homes` | `homes.go` |
| RefreshAccessToken | `GET /v1.0/m/token/{refresh_token}` | `auth.go` |
| ResetDeviceFactory | `PUT /v1.0/devices/{device_id}/reset-factory` | `devices.go` |
| SendDeviceCommands | `POST /v1.1/m/thing/{device_id}/commands` | `devices.go` |
| StartRTCSession | `POST /v1.0/m/life/ipc/{device_id}/webrtc/session` | `client_rtc.go` |
| StopRTCSession | `DELETE /v1.0/m/life/ipc/{device_id}/webrtc/session/{session_id}` | `client_rtc.go` |
| UpdateDeviceFunctionName | `PUT /v1.0/devices/{device_id}/functions/{function_code}` | `devices.go` |
| UpdateDeviceName | `PUT /v1.0/devices/{device_id}` | `devices.go` |
| UpdateDeviceUser | `PUT /v1.0/devices/{device_id}/users/{user_id}` | `devices.go` |
| UpdateMultiOutletName | `PUT /v1.0/devices/{device_id}/multiple-name` | `devices.go` |
| ValidateLoginCode | `GET /v1.0/m/life/home-assistant/qrcode/tokens/{login_code}` | `auth.go` |

The `api/mqtt.asyncapi.yaml` inventory defines `ownerEvents`
(`{ownerTopic}`), `deviceStatus` (`{deviceTopic}/sta`), and `deviceLocal`
(`{deviceTopic}/pen`). `mqtt.gen.go` generates all three templates.
`messages.go` subscribes to the first two and unsubscribes from device status
through typed wrappers. The third is only formatted by `getDeviceTopic`; it is
not currently subscribed. The MQTT protocol 4 and 20 envelope, status and
nested management business fields are generated from `api/openapi.yaml` and
used by `messages.go` and `message_protocol.go`. RTC uses the two HTTP routes
above; the injected alternate signaling interface has no built-in wire route.

`tools/wireroutes` rejects an unknown path, a changed method with the same
path, unknown operation/channel names, and direct Subscribe/Unsubscribe calls.
`TestEncryptedClientRejectsUnschematizedOperation` separately rejects an
unknown route, wrong method, and unexpected query on exported raw helpers.
I reran these negative tests, generation, and tracked generated-file drift at
the reviewed implementation commit. The same generation and route gates passed
in the `v0.3.1` exact-tag workflow.

## Checks at the reviewed implementation commit

- `make lint`, `make check`, `make replay`, and `make coverage`: passed locally.
- `go test ./tools/wireroutes -run TestGateRejects -v`: all negative cases passed.
- `go test ./pkg/tuya -run 'TestEncryptedClientRejectsUnschematizedOperation|TestSyntheticMessageQueue(StartReportsConnectionFailure|StopWaitsForReconnect)' -count=10`: passed.
- The error classification, conflicting option, and blocked-callback delivery synthetic tests passed on ten repeated runs.
- `make generate-wire` followed by `git diff --exit-code` on all three generated wire files: no drift.
- CI and Documentation for `1123ac3`: passed. I independently downloaded the resulting Pages artifact; `tools/check_site_links.py` found 773 valid internal links across 46 rendered pages. All 17 external destinations returned HTTP 200.
- `api/openapi.yaml` declares the default US cloud server, three other regional cloud servers, and QR-specific authentication server overrides. All 28 rendered operation descriptions qualify the cURL as a route illustration. I inspected Fumadocs' `use-server.js` and `operation.js`: SSR uses `example.com` while the hydrated client resolves the selected schema server. A coordinator separately observed the correct hosts in the live hydrated browser; the reviewer browser surface was unavailable, so the hydration evidence is code inspection plus that separate observation.
- `v0.3.0` pointed to the provisional report commit `e30b712` but failed release preflight. Its workflow used Go 1.24 with `GOTOOLCHAIN=local`; `oapi-codegen/v2@v2.8.0` requires Go 1.25. The publish job was skipped, and no GitHub Release was created for that tag.
- `v0.3.1` points to `37584e2`, which changed only the release workflow's Go selection. [Release run 36489207604](https://github.com/portpowered/go-tuya/actions/runs/36489207604) passed both verify and publish. Verify passed regeneration/drift and the route/channel gate, 82.2% coverage (1,302/1,583), module configuration, pre-v1 compatibility policy, build, race, vet, replay, and fresh public Go-proxy consumer compilation. [The published release](https://github.com/portpowered/go-tuya/releases/tag/v0.3.1) and versioned Go Reference return HTTP 200. I reread the corrected release copy: it lists the nine removed exported legacy types, explains the failed `v0.3.0` tag, preserves the provider-evidence limit, and links the `v0.2.0...v0.3.1` changelog and published guides.

## Findings disposition

Earlier handwritten operation dispatch, unsubscribed-channel gate, unused
raw wire structs, invisible initial/reconnect errors, untested queue lifecycle,
and separate QR example guide were corrected at `d376de5`. Coverage reporting
was added at `6832ae0`; typed errors, conflict validation, and callback
backpressure were added at `c4f30aa`. The published request examples were
qualified and schema server defaults added at `1123ac3`. I retested those
changes above. The `v0.3.1` exact-tag gate and corrected release copy close the
remaining findings. The final signoff commit changes only this report and the
library checklist relative to the release tag.
