# Independent review record

At the user's request on 2026-10-04, final dual review and checklist closure
are deferred while known customer-facing fixes, required CI, and releases
are completed. These reports audit earlier snapshots and do not approve
the final release. Unverified checklist items remain open.

Status: **open; findings and final dual approval remain outstanding**.

The two reports below independently audit all 16 checklist items at source
`261ddf44425f3072b14033bd47c36dd11bdbff6c`, against shared template
`62cc3cb5a1308dae8700f92052f99b1c455a1d98`.
They were delivered before either reviewer read this record or the other report.
Later implementation changes require verification on the final commit.
See the [current checklist](template-checklist.md). Historical reports remain
in Git history. Passing CI does not resolve the findings below.

## Reviewer 1 — initial independent audit

### Independent review 1 — initial Tuya library audit

**Reviewed source:** `261ddf44425f3072b14033bd47c36dd11bdbff6c` (the isolated archive provided for this review; its local synthetic Git HEAD is `b8569677079a4941a744e957dad28809e0baa6f9`). The coordinator's manifest confirms the archive contents match the reviewed source tree.

**Review date:** 2026-10-04, America/Los_Angeles.

**Reviewer:** Reviewer 1, independent of implementation.

**Scope:** All 16 items in `docs/template-checklist.md`, with the shared template library, verification, and client-design guidance. This is an initial review, not release sign-off. `docs/independent-review.md` and repository history were intentionally deferred until this initial all-16 report was delivered.

#### Verdicts

| # | Verdict | Evidence and disposition |
| --- | --- | --- |
| 1 | **PASS** | The public API is in `pkg/tuya`; examples are separate and the README presents a standalone SDK. The CLI is in its own module. I found no consuming-application adapter or rollout plan in this source snapshot. |
| 2 | **OPEN — finding R1-2** | The guides and README describe supported methods and distinguish provider-documented from implementation-derived behavior. Fixture provenance says the checked-in exchanges are synthetic, and there are no private account captures. However, `api/` contains no OpenAPI `example`/`examples` entries, and CI/tools contain no validator that validates examples against their owning schemas. Synthetic replay JSON under `tests/replay/fixtures/` is not in the canonical schemas. This misses the checklist's schema-backed request, response, event, nested-payload, resource-update, relation-change, and failure examples plus CI validation. Add the canonical examples and an offline schema-validation gate; retain the current synthetic classification. |
| 3 | **PASS** | `README.md` has Go version, CI, coverage, release, Go Reference, license, and documentation badges, all pointing at the `portpowered/go-tuya` project or its live reports. |
| 4 | **OPEN — findings R1-4a/R1-4b** | `docs/wire-model-inventory.md` records HTTP operations, MQTT channels/Paho framing, generated model groups, primitive values, projections, and handwritten JSON populations. `make wire-routes` passed. The gates test unreferenced exported JSON models and anonymous nested objects. The schema variant binding and the HTTP authority source gate still have gaps: (a) `api/models/events.yaml` binds `RawSharingMessage.protocol` to values 4 and 20 but binds `data` to the single combined `RawSharingMessageData`; separate `RawDeviceReportData` and `RawDeviceManagementData` components are not discriminated/referenced from that envelope. `api/mqtt.asyncapi.yaml` therefore exposes one merged payload instead of the two known protocol variants, while `pkg/tuya/message_protocol.go` branches on protocol and decodes them separately. Schema and generated reference need an explicit variant binding and required shape. (b) A compile-valid mutation `target.Host = "attacker.invalid"` inserted after `requestURL(...)` and before `http.NewRequestWithContext(..., target.String(), ...)` was accepted by `validateSourceNetworkBoundary` in a temporary probe. The boundary checks route/method provenance and tracks request mutation, but does not preserve or validate the `target` URL identity between URL construction and request construction. Track the URL object's effective authority and all request identity fields through construction/send; add this negative control to the exact default gate. Both probe files were removed after use. |
| 5 | **PASS for this source snapshot** | `.golangci.yml` has literal `linters.default: all`, and CI pins golangci-lint v2.14.0, runs full-repository lint and separate CLI/example lint, then build, race tests, vet, formatting, module checks, coverage, and replay. Exact-source CI at `261ddf…`: `verify` and `generation` succeeded (run `37251628188`). I independently ran `make lint` and a clean rerun of `make check` with Go 1.26.8, the pinned v2.14.0 executable, and isolated caches; both passed. The first `make check` overlapped a temporary probe test I removed before its test phase and failed only because that file had been removed. The clean rerun passed all gates. `make replay` also passed. Golangci reported only deprecation warnings for configured `wsl`, `exhaustruct`, and `gomodguard`; there were zero issues. The separate Docs job failed, as recorded under item 12; that does not change the successful Go verification job. |
| 6 | **PASS at the enforced floor; 90% target outstanding** | `make coverage` reports HTTP transport 93.8% (60/64), MQTT transport 50.0% (1/2), public Tuya package 82.8% (1311/1584), and combined non-generated `pkg` coverage 83.2% (1372/1650). This exceeds the CI-enforced 80% floor and remains below the stated 90% target. Generated files are filtered by `tools/coverage`. |
| 7 | **PASS for source layout and inventory** | Public SDK is `pkg/tuya`, generated wire models are grouped in `pkg/dependencymodels`, and HTTP/MQTT transport packages live under `pkg/dependencies/`. There is no catch-all internal model bucket. `make check` passed wire/public-model inventory drift checks; inventory tests cover unreferenced exported JSON structs and anonymous nested wire objects. The CLI and authentication example are separate modules importing the public SDK path. The exact published consumer-proxy check is a release-only check and remains pending publication. |
| 8 | **OPEN — finding R1-8** | `NewClient` uses functional options, has defaults, requires the application client ID, checks conflicting HTTP options, rejects a shared cookie jar, and tests invalid/conflicting cases. But `validateEndpoint` checks only scheme, host, and HTTP(S); it accepts endpoint URLs containing user information, query, or fragment. The later `httptransport.validateOrigin` rejects these when an operation runs, so `NewClient` can accept a configuration that will fail only on first use. Reject those components at option/constructor time and add tests. |
| 9 | **PASS** | `Client` stores reusable endpoint/transport settings; each `Session` owns tokens, HTTP client value, queue, and connection lifecycle. Shared injected `http.Client.Jar` values are rejected, sessions receive cloned clients, and cookie-isolation tests assert two-session behavior. Refresh does not mutate shared client state. |
| 10 | **PASS for implemented network edges** | HTTP is injected through `WithHTTPClient`/`WithHTTPTransport`; MQTT can be replaced with `WithMQTTClientFactory` and a connection-producing Paho hook; RTC signaling is replaceable. The pinned Paho v1.5.1 contract has a `net.Pipe` paired-frame success and denied-CONNACK replay. The route/network gate scans `pkg`, `cmd`, and `examples` and rejects direct socket/network edges. |
| 11 | **PASS** | `AuthService.RefreshToken` is explicit and returns rotated tokens. Sessions expose `Tokens`/`SetTokens`; normal requests do not update tokens. README and authentication guide explain caller persistence and reauthentication responsibility. |
| 12 | **OPEN — findings R1-12a/R1-12b** | Customer guides are MDX under `docs/guides/` and link to generated reference pages. On the exact reviewed source, Docs run `37251628232` failed its rendered-link gate: `provider-evidence/index.html` had broken `./device-list` and `./authentication` links. The source checker reads every rendered HTML page, but its schema-URL extraction only matches `https://portpowered.github.io/...` (`tools/check_site_links.py` line 36); it does not enumerate the many `externalDocs.url` values for `developer.tuya.com` in `api/http.openapi.yaml`, so schema-supplied destinations absent from static anchors are not checked/reviewed by that mechanism. The sibling-link fix and a subsequent Docs run are post-snapshot evidence and need reviewer verification on the final tree/artifact before closing this item. |
| 13 | **OPEN** | README copy is caller-focused; maintainer inventory, verification, release, and checklist material is outside guide navigation. I reviewed the customer guides and maintainer docs, excluding the explicitly deferred independent review record. I reviewed the generated wire inventory at a high level and its drift gates passed, but I did not manually cross-check every inventory row. I could not inspect rendered Pages because the reviewed Docs build stopped at the broken-link gate. Complete the deferred record audit, manually verify the inventory, and inspect the final rendered site before sign-off; check incoming links and release URLs at that commit. |
| 14 | **OPEN** | This is one independent review. The source checklist remains unchecked, and multiple findings/items remain open. A second independent review, finding resolution, affected-check reruns, and both reviewers' verification at the final commit are still required. |
| 15 | **OPEN — finding R1-15** | HTTP and MQTT tests use synthetic paired exchanges, mismatch controls, and exhaustion checks. A temporary compile-valid replay probe built the expected QR request from `qr-created.synthetic.json`, set `request.Host = "attacker.invalid"`, and called `matchFixtureRequest`; the matcher accepted it. At `tests/replay/replay_test.go:137`, the comparison uses URL scheme/host/path only, ignoring the effective `Request.Host`. It also calls `request.URL.Query()` without rejecting malformed raw query errors, and `matchFixtureHeaders` checks expected headers without rejecting unlisted headers. Add authority, URL user-info/opaque, malformed-query, complete relevant-header, and secret-redacted mismatch controls; remove any probe artifacts (already removed here). |
| 16 | **PENDING RELEASE; source review open** | The standalone `cmd/go-tuya` module consumes the public SDK, offers QR login/poll, explicit refresh/export/logout, discovery/read/control, JSON output, cancellation, and offline paired CLI tests. `make check` builds/tests/vets/tidies the CLI module; CI and release workflows pin all-linter v2.14.0. The local `replace ../..` is development-only and the release workflow rejects it, then checks the published CLI from a clean proxy consumer. Neither the SDK nor CLI v0.4 release/proxy install has occurred at this reviewed source, so external install/tag verification is pending. The CLI guide also needs reviewer confirmation against the required short ordered install/login/discovery/operation sequence and a clear explanation of selecting IDs from discovery output. The provider path is QR approval; no browser/localhost/PKCE contract is established by the reviewed Tuya schemas, so I did not count OAuth callback features as missing. |

#### Findings and dispositions

| Finding | Severity | Evidence | Disposition at reviewed source |
| --- | --- | --- | --- |
| R1-2 | P1 | Canonical API schemas contain no `example`/`examples` objects; no schema-validation CI gate found. | Open. Add schema-valid request/response/event examples and validate them in CI. |
| R1-4a | P1 | Protocol 4/20 share one `RawSharingMessageData` reference; specific payload components are disconnected from the message schema. | Open. Bind protocol discriminators to generated named payload variants and check rendered shape. |
| R1-4b | P1 | Temporary source-gate probe mutating `target.Host` after `requestURL` was accepted. | Open. Track effective URL authority and identity from URL construction through send; add a default-root negative test. |
| R1-8 | P2 | `validateEndpoint` accepts endpoint URL user-info/query/fragment even though later origin validation rejects them. | Open. Reject invalid endpoint components when applying client options. |
| R1-12a | P1 | Exact Docs CI run failed on `provider-evidence` sibling links. | Open at reviewed SHA. Coordinator later reported a doc-only sibling-link patch and successful Docs run; verify that change on the final snapshot/artifact. |
| R1-12b | P2 | `check_site_links.py` schema regex only collects GitHub Pages URLs; provider `externalDocs` URLs are outside that collection. | Open. Enumerate all schema-provided externalDocs/description URLs for review and check internal destinations in the rendered artifact. |
| R1-15 | P1 | Temporary replay probe setting `Request.Host` to `attacker.invalid` still matched and would receive the paired response. | Open. Compare effective authority and add malformed-query/URL-userinfo/header-completeness and redaction controls. |

#### Check results

- `make lint GOLANGCI_LINT=<pinned v2.14.0 executable>` — passed, zero issues; deprecated-linter warnings only.
- `make check GOLANGCI_LINT=<pinned v2.14.0 executable>` — passed on clean snapshot; includes SDK/CLI/example vet/build/race tests, schema gates, and module checks.
- `make replay` — passed.
- `make coverage` — 83.2% combined non-generated package coverage; minimum 80% met, 90% target not met.
- Exact-source CI `verify` and `generation` — success; exact-source Docs build — failure as above.
- Final working tree status was clean. Both temporary probe files were deleted; no commit, merge, or source fix was made.

**Signed:** Reviewer 1 — independent initial review.

**Overall verdict:** **NOT APPROVED**; keep checklist item 14 unchecked. Resolve open findings, review the final rendered site and release evidence, and have both independent reviewers verify all affected items at the final commit.

## Reviewer 2 — initial independent audit

### Independent review 2 — initial 16-item audit

**Reviewed source SHA:** `261ddf44425f3072b14033bd47c36dd11bdbff6c` (identified by the review coordinator; local review metadata was excluded from this first pass).

**Review date:** 2026-10-04

**Reviewer attestation:** Reviewer 2, independent and not an implementer. Signed: Codex reviewer agent, 2026-10-04.

#### Scope and result

I assessed every numbered item in `docs/template-checklist.md` against the designated Tuya checkout. All 16 items were addressed (100% review coverage). Nine pass at this source snapshot; seven remain open. “Open” means the checklist evidence is incomplete or a concrete discrepancy remains; it does not mean the underlying code is necessarily unusable.

The target checkout was read-only. I did not read its `docs/independent-review.md`, Git history, or any other review report before delivering this first verdict, as directed by the coordinator. I did not edit or commit source files.

A scope deviation occurred during the audit: three read commands omitted the required checkout working directory. One briefly read a small prefix of the caller’s main Ring checkout `api/openapi.yaml` and its authentication guide; the other two returned only missing-path errors. Those outputs are excluded from every verdict below. No history, review record, or report was read, and no files were changed. I resumed in the designated Tuya checkout with explicit working directories.

#### Verification performed

- `make lint` passed with Go 1.26.8 and golangci-lint v2.14.0. Root, CLI, and authentication-example lint runs each reported zero issues. The checked-in config uses literal `linters.default: all`; workflows pin v2.14.0 and lint all issues.
- `make check` passed with Go 1.26.8. This reran lint, schema/model/route drift checks, build, race tests, CLI module checks/build/tests/vet, and example module checks/build/tests/vet.
- `make replay` passed, including the paired HTTP replay suite and framed Paho MQTT transcript test.
- `make coverage` passed its 80% minimum. Non-generated coverage was HTTP transport 93.8% (60/64), MQTT transport 50.0% (1/2), SDK 82.9% (1313/1584), and combined 83.3% (1374/1650). The 90% target was not met. Generated `.gen.go` statements are excluded by `tools/coverage`.
- The coverage profile has zero-hit non-generated functions in `pkg/dependencies/mqtttransport.NewClient`, several compatibility `GetRequest` methods, `Session.Unload`, `DevicesService.GetDeviceStreamAllocate`, `EncryptedClient.Put`/`Delete`, event `GetDeviceID` methods, `newMQTTClient`, `queueReconnect`, and `NewCustomerTokenInfo`. These uncovered paths are reported rather than counted as generated code.
- Independent read-only GitHub Actions lookups for exact source SHA 261ddf44425f3072b14033bd47c36dd11bdbff6c in portpowered/go-tuya found blocking CI run [37251628188](https://github.com/portpowered/go-tuya/actions/runs/37251628188), completed successfully. Its verify and generation jobs passed, including all-linter checks for SDK, CLI, and example modules, route/schema drift, race tests, coverage, replay, vet, formatting, and module metadata. The separate Documentation run [37251628232](https://github.com/portpowered/go-tuya/actions/runs/37251628232) failed its rendered-site link check; build succeeded and later upload/deploy steps were skipped. Item 5’s exact-commit blocking-CI evidence is present and passes; the documentation-link failure remains open under item 12.

#### Item verdicts

1. **PASS — repository boundary.** The module is `github.com/portpowered/go-tuya`, the public SDK is under `pkg/tuya`, and the separate CLI and examples consume the SDK. The README and public guides address application configuration without containing an application adapter or rollout plan.

2. **OPEN — examples are not schema-backed.** The guides document authentication, operations, errors, transport injection, and evidence classes. Synthetic replay fixtures are stored separately under `tests/replay/fixtures/**/synthetic` with provenance notes. However, the canonical API schemas have no `example`/`examples` entries, and neither `make check` nor the visible CI workflows validate request, response, or event examples against their owning schemas. This leaves the checklist’s schema-valid canonical examples and CI validation requirements unmet.

3. **PASS — README badges.** The README includes Go version, CI, coverage, release, Go Reference, license, and documentation badges. Badge destinations use the `portpowered/go-tuya` repository and Pages coverage/documentation endpoints.

4. **OPEN — known message variants and provenance controls.** `docs/wire-model-inventory.md` is generated, records endpoint/model use sites and primitive bindings, and `make check` passes the model and route drift gates. The protocol inventory is not complete enough to sign off:
   - `api/models/events.yaml` binds `RawSharingMessage.protocol` to the values 4 and 20, but `data` refers to open `RawSharingMessageData`. The known `RawDeviceReportData` and `RawDeviceManagementData` schemas have no incoming schema reference (also reflected by the inventory), so the generated event reference does not bind either protocol value to its concrete payload shape.
   - Several checklist-required compile-valid regression controls are absent from the checked-in test suite: a novel fixed wire value returned through a named result; a generated map returned through a named result and escaping to an unverified helper; a sibling-file helper that returns a novel fixed value or mutates a generated map with a caller-input positive; a helper chain beyond the traversal boundary; nested closed-enum values in arrays/slices and anonymous/reference chains; and indexed receiver-path negatives covering intermediate keys through slices, pointer dereferences, and type assertions. The implementation contains provenance traversal code, but those specific controls are not demonstrated by the tests I inspected.
   - The existing wire and route negative tests run in the repository test suite, but I found no control that mutates a fixture and exercises it through the exact default `make wire-routes`/CI command.

   Relevant source: `api/models/events.yaml`; `docs/wire-model-inventory.md` entries for `RawSharingMessageData`, `RawDeviceReportData`, and `RawDeviceManagementData`; `tools/wiremodels/*_test.go`; `tools/wireroutes/main_test.go`.

5. **PASS — exact-commit blocking CI.** The independent GitHub Actions lookup found CI run [37251628188](https://github.com/portpowered/go-tuya/actions/runs/37251628188) on the exact source SHA, completed successfully with both verify and generation jobs passing. The full-repository SDK, CLI, and example all-linter steps and the build, schema/route checks, race tests, coverage, replay, vet, formatting, and module-metadata steps succeeded. The separate Documentation run [37251628232](https://github.com/portpowered/go-tuya/actions/runs/37251628232) failed its rendered-link check and is tracked under item 12.

6. **PASS AT MINIMUM — coverage.** CI and `tools/coverage` enforce the 80% combined non-generated floor; this snapshot is 83.3%. The 90% target remains unmet. See the zero-hit non-generated paths listed under “Verification performed.”

7. **OPEN — package layout passes; public release consumer remains pending.** Provider wire models live in `pkg/dependencymodels`, public projections are separately schema-generated, and transports live under `pkg/dependencies`. The CLI is a separate module and compiles against `pkg/tuya`, but its `go.mod` uses a local `replace` to the checkout. A clean consumer against the planned public v0.4 SDK module has not been verified because that release is unpublished.

8. **PASS — functional options and client configuration.** `NewClient` uses functional options with endpoint, region, HTTP, MQTT-factory, and RTC-signaling configuration; it validates nil/invalid configuration. Account token state is assigned to sessions, not the reusable client.

9. **PASS — account/session isolation.** The reusable client snapshots the injected HTTP client and rejects a shared non-nil cookie jar. `client_cookie_isolation_test.go` exercises two account sessions and checks complete requests, cookies, tokens, and transport identity. Tokens and MQTT/RTC lifecycle state are session-owned.

10. **PASS — network injection.** HTTP uses an injected `http.Client`/RoundTripper; MQTT creation is replaceable and exposes Paho’s connection-producing hook; RTC signaling is replaceable through `RTCSignaling`. `mqtt_framed_replay_test.go` drives Paho through `SetCustomOpenConnectionFn` and `net.Pipe` and checks frames and teardown. The route gate scans `pkg`, `cmd`, and `examples` for unregistered network edges.

11. **PASS — explicit token renewal.** SDK token exchange/refresh methods return credentials; sessions expose and accept caller-owned token updates. README and authentication guide instruct callers to persist rotated tokens and do not describe silent refresh.

12. **OPEN — source snapshot has broken guide links.** Customer guides are MDX and the Docs workflow builds Fumadocs and checks internal links across rendered HTML plus schema-supplied site links. In this reviewed source, `docs/guides/provider-evidence.mdx` links to sibling pages with `./device-list` and `./authentication`, which resolve beneath the current page. A separate doc-only commit and successful Docs artifact were reported by the coordinator after this source SHA; they are outside this verdict and need re-review on the later snapshot.

13. **OPEN — documentation is not release-ready at this source.** The README is focused and the contributor docs keep inventories and verification detail out of the customer navigation. However, `docs/guides/upgrading.mdx` is presented as “Upgrade to v0.4” while the SDK v0.4 is planned and unpublished; the README marks only the CLI release as pending. `docs/guides/cli.mdx` also omits the implemented `auth logout` command. The exact rendered Pages artifact for this source was not reviewed. I deferred reading the independent review record until after this first 16-item delivery, so final tracked-document review remains incomplete.

14. **OPEN — dual final review is not complete.** This is the independent reviewer 2 verdict for the supplied source only. Both reviewers must verify all fixes at the final commit, all other checklist items must be closed, and the current repository review record must be completed before item 14 can pass.

15. **PASS — paired replay controls.** Synthetic HTTP exchanges match outbound requests before returning responses and fail on unexpected/mismatched exchanges. The pinned Paho v1.5.1 contract is separate in `api/external/paho-mqtt-v1.5.1.yaml`; framed synthetic success and denied-CONNACK replays run through `net.Pipe`, check packet ordering/IDs, and require EOF after teardown. Fixture provenance is explicitly synthetic.

16. **OPEN — CLI is implemented, publication and customer workflow evidence remain incomplete.** The separate CLI module includes QR authentication, explicit refresh/import/export/logout commands, home/device discovery, status/specification reads, explicit device commands, JSON output, cancellation, and offline paired HTTP/MQTT tests. The guide states that the first CLI release is pending and uses a `vX.Y.Z` install placeholder; the module still has a local SDK `replace`, and the release workflow rejects that directive. There is no published CLI-module consumer installation evidence yet. The guide does not document `auth logout` or explain how to select discovered IDs; device control requires the caller to supply a device-specific code and a JSON value via file/stdin. The supported Tuya login path evidenced in these schemas and guides is QR approval/polling; I found no SDK or schema for a browser callback/PKCE flow, so browser-listener requirements are not asserted as supported here.

#### Open findings and disposition

- **F1 — canonical schema examples and validation (items 2, 4): OPEN.** No fix or waiver evidenced in this source review.
- **F2 — MQTT payload variant binding (item 4): OPEN.** Bind protocol 4/20 to generated payload variants and review the rendered reference.
- **F3 — provenance regression-control gaps (item 4): OPEN.** Add the specifically required compile-valid negatives and caller-input positives, then run them through the default gate command.
- **F4 — exact blocking CI (item 5): CLOSED / PASS at this source SHA.** Read-only lookup found CI run [37251628188](https://github.com/portpowered/go-tuya/actions/runs/37251628188) on exact SHA 261ddf44425f3072b14033bd47c36dd11bdbff6c; both jobs passed. The separate Documentation run [37251628232](https://github.com/portpowered/go-tuya/actions/runs/37251628232) failed its rendered-link check, which remains F6/item 12.
- **F5 — published SDK consumer (item 7): OPEN.** Verify the planned v0.4 public module from a clean consumer when the version is published.
- **F6 — broken provider-evidence links (item 12): OPEN at this SHA.** Coordinator-reported doc-only commit `0f6366cca5be055ac58abf5af71c198497e7605d` has a successful Docs run/artifact, but it is a later source and must be checked on the refreshed snapshot.
- **F7 — premature upgrade guide and incomplete CLI guide (items 13, 16): OPEN.** Make release state explicit, document logout and a copyable customer sequence/ID selection, and re-review rendered pages.
- **F8 — final independent review and CLI publication (items 14, 16): OPEN.** Re-audit fixes on the final source and complete the separate public CLI consumer/release checks.

**Signed:** Reviewer 2 (independent, non-author) — 2026-10-04.
