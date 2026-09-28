# Independent standards verification: go-tuya

Reviewer: independent Codex reviewer (not an implementer of the Tuya migration)

Reviewed implementation commit: `1123ac3088a8b535267997330d7187f565cc162b` (2026-09-28).
This is a **working review**. Items marked open have not been signed off. A final
review must name the release tag commit, exact-tag workflow results, and the
later documentation-only signoff commit, if those are different.

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
| 3. Badges | Verified for the existing release | README has Go version, CI, coverage, release, Go Reference, license, and Documentation badges with this repository's destinations. `v0.2.0` and its public package are published. Recheck the badge and latest release URL after the next tag. |
| 4. Complete schema-generated wire surface | **Open** | See the operation/channel inventory and negative tests below. OpenAPI and AsyncAPI, generated descriptors, parameter and header models, encrypted envelopes, nested MQTT payload models, and call sites were inspected. The exported raw encrypted methods now reject method/path pairs outside the generated inventory. Unused handwritten raw/legacy response models were removed. The exact-tag generation, inventory, coverage, and release workflow have not run for these changes. |
| 5. Offline checks and fixture provenance | Verified locally | `make lint`, `make check` (build, route gate, race tests), and `make replay` passed at the reviewed commit. `tests/replay/fixtures/**/synthetic` has provenance notes; no captured fixtures are claimed. Current live account tests remain undocumented. |
| 6. Synthetic coverage | Verified | `make coverage` measured 82.2% (1,302/1,583) non-generated statements, above the CI 80% floor; `tools/coverage` excludes `*.gen.go`/generated markers and reports package and combined values. Synthetic tests exercise queue failure, reconnect, status, stop, backpressure, and typed error paths. `docs/guides/testing.mdx` and the checklist report remaining low-coverage MQTT retry/expiry branches and unimplemented placeholders. The 90% target is not a hard gate. |
| 7. Package boundaries | Verified locally | Public package is `pkg/tuya`; schema-generated private models/descriptors are `pkg/tuya/internal/wire`; examples and replay fixtures are separate. `v0.2.0` passed a separate public-proxy consumer build for this import path. A fresh consumer check for the breaking next release is required on its tag. |
| 8. Option-based construction | Verified | `NewClient(...Option)`, endpoint validation, HTTP, MQTT, RTC, region, and client-ID options exist and have synthetic tests. `WithHTTPClient` and `WithHTTPTransport` reject duplicates and mutual conflicts in either order; a synthetic test covers both orders. Account tokens remain outside reusable client options. |
| 9. Session state and lifecycle | Verified | `Client` holds reusable options, while `Session` owns tokens, queue, and RTC state. `MessageQueue.Status` exposes connection state/error; `Start` reports initial failure and `Stop` cancels and waits for the loop. Callbacks run synchronously and serially, applying backpressure rather than spawning unbounded work; the events guide states that policy. Deterministic race-enabled tests exercise connection failure, reconnect, stop, and blocked-callback delivery. |
| 10. Transport injection | Verified | `WithHTTPClient`/`WithHTTPTransport` cover auth, encrypted cloud HTTP and built-in HTTP RTC signaling; `WithMQTTClientFactory` and `WithRTCSignaling` cover MQTT and alternate signaling edges. Synthetic HTTP, MQTT, and RTC tests use those seams. No built-in WebSocket endpoint exists. |
| 11. Explicit credentials | Verified | `AuthService.RefreshToken` returns rotated credentials without mutating the session; `Session.Tokens` and `SetTokens` expose caller-managed state. Synthetic refresh tests check the unchanged session until explicit assignment. |
| 12. MDX customer guides | Verified at the reviewed commit | Nine `docs/guides/*.mdx` pages cover important workflows and link generated OpenAPI/AsyncAPI references. QR example instructions moved from `examples/auth/README.md` into authentication MDX; its README is now a pointer. Documentation deployed for `1123ac3`; I downloaded its 46-page artifact and reran the full link checker (773 internal links passed). |
| 13. Concise published copy | Verified before the next release | I inspected the rendered root, guide, 28 OpenAPI, and three AsyncAPI pages. All 28 operation descriptions label generated cURL as a route illustration and point to the signed/encrypted client flow; global cloud servers and QR-specific auth servers match client defaults. The 46-page link checker found 773 valid internal links, and all 17 external destinations returned HTTP 200. The shared Fumadocs client code hydrates the server selection from schema; a coordinator observed the correct cloud/QR hosts in the live hydrated browser. The static prerender still contains Fumadocs' `example.com` fallback, so this review does not treat the static cURL text as a live snippet. Recheck new release notes after tagging. |
| 14. Independent signoff | **Open** | This document records the independent audit, but item 4's exact-tag gate and the next release-note check remain unresolved. Tracking these findings does not sign them off. Re-audit the tag run and docs-only signoff commit before checking item 14. |

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
the reviewed commit; they passed. Final exact-tag release checks remain
required.

## Checks at the reviewed implementation commit

- `make lint`, `make check`, `make replay`, and `make coverage`: passed locally.
- `go test ./tools/wireroutes -run TestGateRejects -v`: all negative cases passed.
- `go test ./pkg/tuya -run 'TestEncryptedClientRejectsUnschematizedOperation|TestSyntheticMessageQueue(StartReportsConnectionFailure|StopWaitsForReconnect)' -count=10`: passed.
- The error classification, conflicting option, and blocked-callback delivery synthetic tests passed on ten repeated runs.
- `make generate-wire` followed by `git diff --exit-code` on all three generated wire files: no drift.
- CI and Documentation for `1123ac3`: passed. I independently downloaded the resulting Pages artifact; `tools/check_site_links.py` found 773 valid internal links across 46 rendered pages. All 17 external destinations returned HTTP 200.
- `api/openapi.yaml` declares the default US cloud server, three other regional cloud servers, and QR-specific authentication server overrides. All 28 rendered operation descriptions qualify the cURL as a route illustration. I inspected Fumadocs' `use-server.js` and `operation.js`: SSR uses `example.com` while the hydrated client resolves the selected schema server. A coordinator separately observed the correct hosts in the live hydrated browser; the reviewer browser surface was unavailable, so the hydration evidence is code inspection plus that separate observation.

## Findings disposition

Earlier handwritten operation dispatch, unsubscribed-channel gate, unused
raw wire structs, invisible initial/reconnect errors, untested queue lifecycle,
and separate QR example guide were corrected at `d376de5`. Coverage reporting
was added at `6832ae0`; typed errors, conflict validation, and callback
backpressure were added at `c4f30aa`. The published request examples were
qualified and schema server defaults added at `1123ac3`. I retested those
changes above. The exact-tag release check remains open; this review does not
sign off item 14 while that gate is pending.
