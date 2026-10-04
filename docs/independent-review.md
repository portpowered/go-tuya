# Independent review record

Status: pending two independent reviews of all sixteen items in the
[current checklist](template-checklist.md) at the final implementation commit.
Record separate verdicts, evidence, findings, and dispositions here, including
exact CI, documentation, and published SDK/CLI installation results.

## Historical scope

Earlier verification covered the previous fifteen-item standard. Its latest
review targeted `264a30c9669c0a25d8a73a2968ec37b52225f663`; those verdicts
do not certify expanded model-generation, documentation, or CLI requirements.
The original records remain in Git history:
[itemized review](https://github.com/portpowered/go-tuya/blob/60f909970ec6afe589bd2c862709f79927acbd8b/docs/independent-review-264a30c.md),
[earlier verification](https://github.com/portpowered/go-tuya/blob/60f909970ec6afe589bd2c862709f79927acbd8b/docs/independent-verification.md),
and [paired replay review](https://github.com/portpowered/go-tuya/blob/60f909970ec6afe589bd2c862709f79927acbd8b/docs/paired-replay-review.md).

## Reviewer 1 — exact commit 48d4f393e07d5ad6768d796db90a600b31cd645c

Independent review against all sixteen items in `template-checklist.md` and
the shared standards pinned at
`987b9c34a6b927472c21604617b6842a4238746b`. I did not implement the Tuya
changes and made no SDK or CLI source edits. Source inspection used an archive
of the exact reviewed commit; the working tree's pre-existing untracked
coverage scratch files were left untouched. This is a provisional R1 record,
not sign-off: findings F1–F9 remain open and Reviewer 2 has not yet recorded a
Tuya section.

### Exact verification evidence

- CI run [37164616409](https://github.com/portpowered/go-tuya/actions/runs/37164616409)
  completed successfully at the reviewed SHA. Its `generation` and `verify`
  jobs passed, including all-linter runs for the root, CLI, and auth example
  modules; generation drift, build, race tests, coverage floor, replay, vet,
  formatting, and module checks.
- Documentation run
  [37164616482](https://github.com/portpowered/go-tuya/actions/runs/37164616482)
  completed successfully at the same SHA. It built the Fumadocs site, checked
  811 internal links across 46 rendered pages, and uploaded artifact
  [11289435896](https://github.com/portpowered/go-tuya/actions/runs/37164616482/artifacts/11289435896).
  I inspected the artifact; the customer guides and generated references were
  rendered and visibly labeled implementation-derived. The `deploy` job was
  skipped because this was a pull request. The currently published
  `/docs/guides/cli/` route returns 404, so the candidate site's publication
  is still unverified.
- The public SDK proxy reports versions through `v0.3.4`; `v0.3.5` is not
  available. The reviewed `cmd/go-tuya/go.mod` requires `v0.3.5` but retains
  `replace github.com/portpowered/go-tuya => ../..`. The nested tag
  `cmd/go-tuya/v0.1.0` is absent, and the public proxy lists no CLI versions.
  No SDK or CLI release workflow has published this candidate.

### Item verdicts

1. **PARTIAL — F1.** I found no PortOS import or application adapter. However,
   the public provider package exposes `QueryDevicesByHomeAssistantDevices`
   and defaults to the Home Assistant client ID and `haauthorize` schema in
   `pkg/tuya/constants.go`; the HA-named method duplicates the generic home
   device operation. This may be a protocol requirement, but the repository
   does not distinguish that from an application adapter. Resolve the scope
   and document or remove the app-specific surface before certifying
   application independence.
2. **PASS.** `README.md` and the authentication, device, control, event, and
   CLI guides describe the public operations, errors, credential ownership,
   and supported injection options with matching API examples. They separate
   synthetic evidence from the maintainer's unrecorded account tests and
   disclose placeholders and unsupported local control. The README also
   documents timeout and token responsibilities.
3. **PASS.** `README.md` contains Go version, CI, coverage, release, Go
   Reference, license, and documentation badges with this repository's live
   targets. I checked the coverage endpoint/report, published guide index,
   `pkg.go.dev`, and linked reference destinations; they responded
   successfully. The README's pinned install example is stale; that is tracked
   under item 13.
4. **OPEN — F2.** Schema responsibility groups and generated route/model files
   exist, and CI checks generated drift, including untracked generated model
   files. But there is no complete checked-in wire-model inventory and no Go
   source scan that proves every active or legacy wire definition is generated.
   `tools/wiremodels` checks schema ownership/references; its tests do not add
   an unreferenced exported handwritten JSON struct, an anonymous wire object,
   or a novel nested payload/key. Concrete value gaps remain: handwritten
   `ProtocolDeviceReport=4` and `ProtocolOther=20` duplicate generated
   `wire.N4`/`wire.N20`; `BizcodeOnline`, `BizcodeOffline`, and related event
   codes are recognized in `pkg/tuya/message_protocol.go` while the schema's
   `bizCode` remains an open string with no generated known-value bindings;
   and the capability table hard-codes device status codes such as
   `switch_led`, `bright_value`, and `colour_data` while the generated status
   model has only an open string code. `ParseEvent` also reads the known
   `protocol` and `data` wire keys directly from a map. These known values and
   parser keys need schema ownership/bindings and negative drift tests while
   retaining genuinely open device codes.

   `api/mqtt.asyncapi.yaml` inventories dynamic application topics but not the
   active Paho v1.5.1 connection protocol, broker handshake, or source-matched
   framed traffic. The pinned dependency's network exchange and callsite are
   therefore not represented by the required separate external protocol
   contract.

   The route gate in `tools/wireroutes/main.go` scans only `pkg/tuya/*.go` and
   checks source text/presence of generated selectors rather than proving the
   method and transformed path at each actual send. The encrypted path does
   fail closed through `wire.IsKnownOperation` in `EncryptedClient.makeRequest`,
   but the two direct auth sends in `pkg/tuya/auth.go` bypass it. The
   source-checker accepts request construction based on a generated method
   argument and does not bind the URL argument to the matching generated route;
   its only mismatched-method negative test does not test a changed URL after
   construction. The `wire` identifier checks do not resolve import identity
   or lexical shadowing. Add callsite/dataflow negatives for direct auth URL
   mutation, aliases/shadowing, and generated-path transformations.
5. **PASS.** The exact blocking CI run above passed. `.github/workflows/go.yml`
   pins golangci-lint v2.14.0, keeps all linters enabled, and covers all
   repository Go modules; no issue suppression or continue-after-failure path
   was found in the relevant jobs.
6. **PASS ON THE FLOOR; 90% TARGET NOT MET.** Exact CI measured 82.7%
   non-generated coverage for `pkg/tuya` (1,332/1,611 statements), enforcing
   the 80% floor. The coverage tool excludes generated files and the generic
   `pkg/testing` subtree. The public and transport implementation currently
   occupy the same package, so there is no separate transport-package figure.
   Report the shortfall from the 90% target and review uncovered behavior
   before release; do not describe the current result as 90%.
7. **OPEN — F3.** The schema sources and generated models are split by
   authentication, homes, devices, message queue, events, RTC, encryption, and
   common responsibilities. The public package is under `pkg/tuya`, and
   generated contracts are under `pkg/dependencymodels`. However, transport
   behavior remains mixed into `pkg/tuya` (`auth.go`, `encryption.go`,
   `messages.go`, and RTC signaling) rather than a transport package, and the
   required full inventory mapping schema component, generated type, generator,
   transport use, and conversion callsite is absent. Public projections in
   `api.go`, event projections, custom decoders, primitive constants, and
   compatibility exports therefore have not been dispositioned entry by
   entry. The separate CLI module is only tested against a local replacement;
   no public consumer proof exists yet.
8. **PASS.** `NewClient` uses functional options, validates nil/conflicting
   transports and endpoint values, and supplies defaults. Client configuration
   is separated from account credentials in `Session`; callers can inject the
   HTTP client/transport, MQTT client factory, and RTC signaling.
9. **PASS.** Tokens and message-queue lifecycle state are session-scoped;
   `Session.Close` stops its queue, and RTC streams expose stop/cancel behavior.
   Client configuration is reusable across sessions. The MQTT close test only
   proves fake-client behavior; its on-wire limitation is recorded under item
   15.
10. **OPEN — F4.** HTTP is injectable through `WithHTTPClient` or
    `WithHTTPTransport`, and default RTC signaling uses that HTTP client. MQTT
    is a real network edge through pinned `github.com/eclipse/paho.mqtt.golang`
    v1.5.1 (`newMQTTClient` calls `mqtt.NewClient`). `WithMQTTClientFactory`
    receives `*mqtt.ClientOptions`; a caller can set Paho's
    `SetCustomOpenConnectionFn` there and return `mqtt.NewClient(options)`, so a
    connection-producing hook is reachable indirectly. However, no repository
    test exercises that hook or Paho framing over its returned `net.Conn`; the
    replay fake replaces the high-level `mqtt.Client`. Test the actual framed
    exchange and teardown through the Paho hook before certifying this item.
11. **PASS.** `AuthService.RefreshToken` is an explicit operation that returns
    the rotated token pair. `Session.Tokens` and `SetTokens` make storage and
    application explicit; `requiredAccessToken`/`requiredRefreshToken` fail
    when credentials are absent and no implicit refresh was found. README and
    authentication guide tell callers to persist and apply rotated tokens.
12. **OPEN — F5.** The reviewed guides are MDX under `docs/guides/`, and the
    exact Docs build produced the candidate site with all 46 pages and checked
    internal links. Publication is not established because PR deployment is
    skipped and the current live CLI-guide route returns 404. Deploy the exact
    reviewed candidate and verify its guide/reference routes before checking
    this item.
13. **OPEN — F6.** I reviewed all tracked Markdown/MDX and the rendered
    candidate. The README still recommends `go get ...@v0.2.0` and includes
    package-migration history and a contributor check table despite public
    `v0.3.4`. `docs/guides/testing.mdx` reports 82.8% (1,336/1,614), conflicting
    with exact CI's 82.7% (1,332/1,611); it exposes coverage mechanics and
    low-level function coverage in the customer guide. `docs/guides/schema.mdx`
    includes generator commands and source-gate implementation details in
    customer navigation. Consolidate or relocate those maintenance details and
    update the coverage/install copy. The docs workflow's external-link review
    list contained 16 destinations; I checked those rendered destinations and
    they responded successfully. Internal checks alone do not resolve the
    copy/audience findings.
14. **OPEN — F7.** This section records R1's exact-commit review, but no
    independent R2 Tuya all-16 section is present yet. Items 1, 4, 7, 10, 12,
    13, 15, and 16 have open findings; item 14 cannot pass until those are
    fixed and both reviewers verify the final commit.
15. **OPEN — F8.** HTTP fixtures are synthetic paired exchanges. The replay
    matrix covers all 28 schema HTTP operations: 22 ordered pairs are consumed
    by `TestRemainingOperationsPairedReplay`, and the other six auth/home/device
    operations are exercised by separate paired tests. The matcher checks
    method, origin, escaped path, query multimap, expected headers, request
    body/decrypted meaning, signatures, and returns the paired response only
    after request validation; `orderedHTTPReplay` rejects an unexpected next
    request and requires ordered consumption for the operation transcript.
    The MQTT transcript is
    not a framed bidirectional replay: `mqttReplayClient.Connect` records an
    abstract `connect` action, `Subscribe`/`Unsubscribe` record method calls,
    and tests invoke the fake client's publish handler directly. The fixture
    has no CONNECT/CONNACK, SUBSCRIBE/SUBACK, PUBLISH packet, or MQTT-level
    teardown acknowledgement. Thus it does not verify the actual Paho network
    exchange or socket cleanup. The HTTP side of message-queue configuration
    is paired, but it does not close this MQTT evidence gap.
16. **OPEN — F9.** The CLI implementation, MDX guide, separate module, and
    blocking lint/build/race/vet/module CI checks exist. CLI tests use paired
    HTTP fixtures and a fake MQTT client. The module requires unreleased SDK
    `v0.3.5` and has a local `replace`; public SDK versions stop at `v0.3.4`,
    and there is no nested CLI version/tag or clean public `go install` result.
    The CLI release workflow describes the future proxy/install check but has
    not run for a published tag. Keep this open until SDK `v0.3.5` and a CLI
    tag are published and verified from a clean consumer.

### R1 finding dispositions at 48d4

F1 app-specific auth identity/API scope — open. F2 incomplete wire-model,
primitive binding, and actual route-callsite inventory/gates — open. F3 missing
transport/model inventory boundary — open. F4 no test of Paho's injectable
connection seam and MQTT framing — open. F5 candidate Pages publication missing — open. F6 stale and
maintenance-heavy customer documentation — open. F7 second independent Tuya
review/final SHA — open. F8 MQTT replay does not exercise framed network
traffic — open. F9 no public SDK v0.3.5 or CLI module release/consumer proof —
open. Do not check the corresponding checklist items based on this record
alone.
