# Independent review record

Status: **open; fixes and two fresh final independent approvals are required**.

The reports below audit `f1ca66afc9d28dbb8c551535bfff2d0b5677eecd`
against shared template `05e93ff08899414207e9335717e7d7b0190ebd09`.
The current [checklist](template-checklist.md) pins
`62cc3cb5a1308dae8700f92052f99b1c455a1d98`.
Historical scoped approvals remain in Git history and do not approve later changes.

## Findings being resolved

- Unknown, recursive, imported, or helper-owned wire values can evade schema checks.
- Generated maps and query values can lose provenance through fields and helper escapes.
- Later closed-enum field mutations can evade validation.
- A new network primitive and post-guard route mutation can evade the route inventory.
- Signature preimage formatting needs schema ownership and generated runtime use.
- Request-body backing storage needs mutation checks through the actual send.
- Old release guide URLs need a rendered compatibility destination.
- An orphan document needs removal and the review record needs consolidation.
- Injected HTTP cookie state needs isolation from callers and other sessions.

The original reviewers became implementers of the fixes, so two new reviewers
must independently audit all sixteen items at the frozen implementation commit.
Every finding remains open until those reviewers verify its disposition. Release,
Pages, clean SDK consumption, and public CLI installation need final evidence.

## Original review 1: repair evidence

# Independent checklist audit — Reviewer 1

**Repository:** portpowered/go-tuya
**Reviewed commit:** f1ca66afc9d28dbb8c551535bfff2d0b5677eecd
**Implementation baseline:** f5506c7f303debd917e13ae1827911909da30de9 (reviewed commit changes checklist wording only)
**PR base:** main
**Shared standards:** portpowered/go-third-party-template at 05e93ff08899414207e9335717e7d7b0190ebd09
**Review date:** 2026-10-04

I reviewed a fresh temporary clone at the exact SHA. Checks ran from its repository root with Go 1.26.8 and scoped temporary caches; the pinned golangci-lint v2.14 executable was used. Temporary adversarial source probes were compiled and removed from that clone. The original go-tuya checkout, private/untracked files, checklist, and review records were not edited.

## Overall determination

**Not merge-eligible at this reviewed commit.** Checklist item 4 has reproducible fail-open source-gate cases: unresolved recursion, generated-map and schema-keyed query-map writes through aggregate receiver fields, and an unlisted x/net/websocket network primitive all pass the exact root make wire-routes gate. Fix those cases and have both independent reviewers recheck the final implementation SHA before considering merge.

The PR CI and docs artifact are green, but they are not a full publication pass. The Docs workflow intentionally skips Pages deploy on pull requests. SDK v0.4 and nested CLI release/consumer-install checks have not run. Those publication checks can occur after merge and before first release; they are distinct from fixing the blocking source gate. Items 14, 12, and 16 therefore remain open/partial even aside from item 4.

## Checklist verdicts

### 1. Application independence — PASS

The library requires the consuming application's client ID through WithClientID; it does not supply an application identity. pkg/tuya exposes a provider SDK, and the CLI is a separate nested module. The README says applications supply and store account credentials. Tuya mobile/private endpoint paths containing home-assistant are provider routes, not an application adapter. No consuming-app import or adapter was found in the public package, examples, README, or guides.

### 2. API usage, operations, errors, injection, customer guides — PASS

The README and MDX guides cover authentication and explicit refresh, device discovery/control, event lifecycle, regions, and the standalone CLI. Examples call the current exported options, session, and service methods. They document ClientError, caller-owned token persistence, transport injection, and lifecycle cleanup. The device-list guide separates Tuya-documented behavior from implementation-derived signing/encryption. Replay provenance is explicitly synthetic, with no account-capture claim. The rendered candidate has guides for authentication, CLI, device control/list, events, regions/endpoints, upgrading, and an index.

### 3. README badges — PASS

README has Go-version, CI, coverage, release, Go Reference, license, and documentation badges. Their targets use portpowered/go-tuya and live report/repository URLs; no template example module or repository placeholder remains.

### 4. Schema/model/network inventory and fail-closed source gates — OPEN, BLOCKING

The checked-in inventory has separate HTTP operations, MQTT/external-edge, generated provider components, primitive-value bindings, public projections, handwritten SDK/CLI JSON types, and codec sections. OpenAPI contains 28 operation IDs across 24 paths; AsyncAPI describes two receive channels. Generated-artifact and route checks pass on the clean target (make wire-routes). The external Paho contract identifies v1.5.1, broker/config source, frames, injection seam, replay fixtures, and EOF lifecycle behavior. I independently searched production Go types, JSON tags, maps, constants, and outbound-network sites in addition to regenerating/checking the inventory.

Compile-valid negative probes show the gate does not meet fail-closed requirements. Each probe was placed in a temporary production file under pkg/tuya, passed go test ./pkg/tuya, and was removed. For aggregate probes I ran go run ./tools/wiremodels to refresh the temporary inventory, then the exact root make wire-routes; the gate passed with the novel value/key still present.

**Reproductions that pass when they must fail:**

1. **Unresolved recursion is treated as acceptable provenance.** Define auditRecursiveWireKey() string { return auditRecursiveWireKey() }, then assign result.AdditionalProperties[auditRecursiveWireKey()] = value for a generated wire.LoginCodeResult. This compiles, and after wiremodels refresh make wire-routes exits 0. It must emit a diagnostic because recursion prevents proving the key is caller-owned.
2. **Generated map escape through aggregate field and receiver method.** Store result.AdditionalProperties in auditWireMapHolder{values: ...}; call holder.Apply(), whose receiver method writes holder.values["audit-unregistered-holder-method-key"] = nil. Refreshed inventory followed by root make wire-routes exits 0. The nested schema-owned map's trust must not survive storage in a cross-function aggregate field.
3. **Schema-keyed url.Values aggregate-field mutation.** Store url.Values{string(wire.QueryParamClientid): []string{"client-id"}} in an aggregate field, then call holder.values.Set("audit-unregistered-aggregate-query", "value"). After inventory refresh, root make wire-routes exits 0. This bypasses generated query-key enforcement through a receiver field.
4. **Unlisted dependency network primitive.** A compile-valid production file importing pinned golang.org/x/net/websocket and calling websocket.Dial("wss://mqtt.example.invalid/socket", "", "https://client.example.invalid") passes go test ./pkg/tuya and root make wire-routes. tools/wireroutes/network.go recognizes Paho and several WebSocket modules, but its package whitelist omits golang.org/x/net/websocket. The gate must reject this unlisted active socket edge.

Controls I also exercised passed as intended: caller-defined open map value/key positive; sibling-file fixed keys and generated-map mutation fail; named-result generated value escape fails; helper/method map writes fail; returned callback writes fail; global generated-map write fails; 128-helper chain and recursive fixed fallback fail. These controls do not resolve the four open cases above.

The failures reproduce through the exact Makefile command from the default root working directory, not isolated helper-unit tests. They conflict with item 4's requirements that unresolved recursion, aggregate map storage, and unlisted dependency traffic fail the gate.

### 5. Blocking all-linter CI — PASS

.golangci.yml sets the literal linters.default: all. Repository CI pins golangci-lint v2.14.0 and runs root, CLI, and auth-example lint without new-issues-only or continue-on-error behavior. Exact PR CI [run 37192250212](https://github.com/portpowered/go-tuya/actions/runs/37192250212) completed successfully on the reviewed SHA; route/generation, lint, build, race, coverage-floor, replay, vet, CLI, and auth-example checks passed. Local root make lint and make check also succeeded; root, CLI, and auth-example lint each had 0 issues.

### 6. Synthetic fixtures and non-generated coverage — PARTIAL

make coverage passed the enforced 80% floor:

- httptransport: 93.8% (60/64)
- mqtttransport: 50.0% (1/2)
- pkg/tuya: 82.8% (1312/1584)
- combined non-generated production packages: 83.2% (1373/1650)

The 90% target is not met. The coverage tool excludes pkg/testing/, files named *.gen.go, and files with the generated header. Remaining uncovered behavior includes pkg/tuya/messages.go:queueReconnect (0%), pkg/tuya/tokens.go:NewCustomerTokenInfo (0%), and pkg/tuya/users.go:Unload (0%); MQTT transport has one of two statements covered. These are reported gaps, not a request to add tests only to raise a number.

### 7. Package/schema boundaries and import-path verification — PARTIAL

The public SDK is under pkg/tuya; generated provider wire models are in pkg/dependencymodels; HTTP/MQTT adapters are in pkg/dependencies/httptransport and pkg/dependencies/mqtttransport. Responsibilities are split across auth, device, event, route, and external MQTT schemas/generated files. Public semantic projections are separately generated from provider wire types.

The release workflow defines a fresh-consumer SDK import test against the public module proxy, but it is tag-triggered and was not run for this PR. There is no v0.4.0 publication yet, so candidate import path has not been verified from a clean external consumer module.

### 8. Functional options and validation — PASS

NewClient uses functional options with region/URL defaults and validation for missing/blank client identity, bad endpoints, nil transports/factories, and conflicting HTTP client/transport options. Account tokens are supplied to sessions rather than stored in reusable client configuration.

### 9. Client/session ownership — PASS

Client contains reusable configuration. Each Session owns a token snapshot, services, message queue, and connection state. Session.Tokens, SetTokens, Close, queue stop, and RTC stream/session close expose ownership and lifecycle operations. Concurrent refresh responsibility is documented for callers.

### 10. Network injection seams — PASS for current SDK edges

HTTP requests use the injected HTTP client/transport. MQTT supports WithMQTTClientFactory; Paho SetCustomOpenConnectionFn can return an offline net.Pipe. RTC signaling can be replaced with WithRTCSignaling; default signaling uses the injected HTTP edge. make replay passed, including Paho framed transcript and close/EOF assertion. Item 4 separately remains blocking because the source gate fails to reject an additional unlisted edge.

### 11. Explicit token exchange/refresh — PASS

AuthService.RefreshToken returns the refreshed access/refresh pair. Callers explicitly persist the pair and call Session.SetTokens; the reusable client does not silently rotate stored credentials. README and authentication guide explain this contract.

### 12. Rendered Pages and all-page links — PARTIAL

Exact Docs run [37192250259](https://github.com/portpowered/go-tuya/actions/runs/37192250259) succeeded at the reviewed SHA and uploaded its rendered artifact. I downloaded and inspected the artifact in TEMP: 43 HTML pages including generated references, guides, root, and not-found routes. The workflow-equivalent link checker, run with GITHUB_REPOSITORY=portpowered/go-tuya, reported **801 internal links across 43 rendered pages**. I inspected rendered guide/reference titles and representative content, including upgrading and reference pages.

There are two unique Tuya externalDocs URLs; I opened both and confirmed they render Tuya documentation pages. The static link checker only collects project Pages URLs from API YAML, so it does not automatically verify schema-supplied Tuya links.

The PR workflow skips deploy and Pages-artifact upload for a pull request. Thus the candidate artifact is verified, but the exact site has not been published from main. The release-note link points to the upgrading guide route present in the artifact.

### 13. Concise, audience-appropriate docs — PARTIAL

README stays focused on installation, authenticated usage, capabilities, configuration/lifecycle, and guide links; inventory and generation detail are in contributor docs. Customer guides distinguish provider evidence and synthetic behavior. Concrete cleanup items remain before a release:

- docs/guides/cli.mdx repeats the credentials/untrusted-endpoint caveat in the final paragraph immediately after the same warning in the preceding paragraph.
- TASKS.md contains only the orphan heading ### NOtes.
- README instructs go get ...@latest, which currently resolves to published SDK v0.3.4 while this candidate documents the unreleased v0.4 API. The CLI guide says its first release is pending; the SDK installation claim should also be version-aware at publication.
- The current independent-review document embeds a lengthy report for old commit 48d4f… while saying earlier records remain in Git history. This is duplicate reviewer evidence in the current document and should be removed or moved into historical Git records.

### 14. Two independent all-item reviews — OPEN

At the reviewed SHA the checklist items are unchecked, and docs/independent-review.md says two independent reviews of all 16 items are pending. This R1 report is separate and in TEMP; it does not alter repo signoff marks. Item 4 remains unresolved and no R2 review of the final fixed SHA exists. Keep item 14 open until both reviewers record item-by-item evidence and recheck the final candidate.

### 15. Paired request/response and ordered frame replay — PASS

The HTTP replay harness checks outbound method, origin, escaped path, repeated query values, relevant headers, and body before returning paired status/headers/body; it rejects unexpected/duplicate calls and verifies expected calls are consumed. HTTP fixtures and provenance notes label cases synthetic. Paho success and denied-CONNACK tests exercise ordered client/server MQTT frames through net.Pipe, validate variable packet IDs, require clean disconnect/EOF, and reject changed frames. make replay passed. The client uses QR login rather than OAuth; OAuth state/PKCE requirements do not apply. No real account captures are claimed.

### 16. Standalone CLI and published install/release — PARTIAL

cmd/go-tuya is a separate module with auth, explicit refresh, home/device discovery, status/spec reads, explicit command, and event-watch commands. Credentials are accepted through protected files, environment, stdin, or explicit files rather than ordinary secret arguments; JSON output/help are documented. go -C cmd/go-tuya run . --help succeeded locally, and exact blocking CI covered CLI lint/build/race tests/vet/module checks. CLI synthetic paired HTTP and MQTT lifecycle fixtures are present.

gh release list shows SDK latest v0.3.4 and no v0.4.0; the repository has no cmd/go-tuya/v* release tag. The nested release workflow requires published SDK v0.4.0 and then verifies a separate consumer module and go install through the public proxy. The first CLI release is correctly marked pending in its guide. Published CLI installation and module tagging remain unverified until after SDK v0.4 publication and nested CLI release.

## Merge/release disposition

- **Before merge:** fix and retest item 4's unresolved-recursion, aggregate-map receiver, and dependency-network cases; rerun affected gates and exact blocking CI on the fixed SHA. Obtain R2's independent all-16 review of that final SHA. Current SHA is not ready to merge.
- **Before first public release:** resolve remaining partial item 6/7 evidence as required by maintainers, publish/deploy exact main Docs site and verify it, clean up item 13 docs, publish SDK v0.4.0, then release/install separately tagged CLI and record fresh-consumer evidence. Passing the current PR Docs artifact is not a substitute for those publication checks.

## Original review 2: repair evidence

# Reviewer 2 — independent go-tuya checklist review

**Reviewed commit:** `f1ca66afc9d28dbb8c551535bfff2d0b5677eecd`
**Repository:** `portpowered/go-tuya`
**Underlying SDK/tool implementation:** `f5506c7f303debd917e13ae1827911909da30de9`; the reviewed head adds the checklist-only change.
**Review result:** **Not merge-eligible yet.** Item 4 has reproducible fail-open source-gate bypasses; item 12/13 have a broken published release-note destination and an orphan tracked document.

I reviewed the full 16-item checklist and the linked shared library, verification, client-design, website, and releasing standards. I used an isolated clone at `C:/Users/andre/AppData/Local/Temp/go-tuya-review-r2`; all probes were temporary and removed. The original checkout was left untouched, including its existing untracked files. The temp clone has no source diff; its only untracked content is the downloaded Docs artifact in `rendered-site/`.

I did not open `docs/independent-review.md`, as directed. This report is outside the repository and does not change any checklist signoff.

## Verification evidence

- Exact frozen-head CI run `37192250212` succeeded on `f1ca66af...`: both `generation` and `verify` jobs passed. The verify job ran SDK, nested CLI, and auth-example all-linter checks, build, schema route gate, race tests, non-generated coverage, replay, vet, formatting, and module metadata checks.
- Exact frozen-head Documentation run `37192250259` succeeded. It regenerated schema artifacts, built the site, checked links, and uploaded `documentation-site`. Deployment was skipped because this was a pull request.
- The earlier implementation-head runs `37191650873` (CI) and `37191650857` (Docs) also succeeded; I checked their successful job/step lists.
- In the isolated clone, `make lint`, `make check`, `make coverage`, and `make replay` all passed with Go 1.26.8 and caches outside the cloned module. `make check` included the SDK, CLI, auth example, generated-model and route gates, race-enabled tests, and module checks. The pinned native golangci-lint v2.14 executable reported no issues; config has literal `linters.default: all`, and CI uses v2.14.0 with `only-new-issues: false`.
- Coverage was 83.2% combined (1,373/1,650 non-generated statements): SDK 82.8%, HTTP transport 93.8%, MQTT transport 50.0%. The documented 80% CI minimum passes; the 90% target is not met and is described as a target.
- I independently compared the generated wire-component table's 68 component names with generated declarations; every table entry had a generated declaration. Seven generated operation-parameter types were listed separately. An independent production-source scan found 108 handwritten JSON-tagged structs under `pkg/`, `cmd/`, and `examples/`; all 108 matched inventory entries, with no missing or extra entries. All 1,155 file/line references in the inventory resolved to existing source lines. Route inventory has 28 HTTP operations and two MQTT channels.
- A clean separate consumer module importing `github.com/portpowered/go-tuya/pkg/tuya` and compiling a call to `tuya.NewClient()` passed after `go mod tidy`; it used a local `replace` to the isolated source. This does not substitute for a published-proxy install.
- I downloaded the exact Docs artifact and inspected all 43 HTML pages, including every guide, the site root, generated OpenAPI operation pages, AsyncAPI pages, and fallback pages. Re-running the checker with `GITHUB_REPOSITORY=portpowered/go-tuya` checked 801 internal links across all 43 rendered pages. The seven user guides plus guide index exist and render expected content.
- I manually followed schema externalDocs to Tuya’s “Get Device List” and “Device Management” pages; both pages resolved to the relevant API reference. GitHub auth/command example destinations and pkg.go.dev returned HTTP 200. The README’s live coverage JSON/HTML, Pages root, and pkg.go.dev targets also returned HTTP 200.
- The exact artifact has no `docs/guides/provider-evidence/` page. The old release-note URL below currently returns HTTP 200 but serves the generic API-reference not-found fallback.

## Checklist verdicts

1. **PASS.** The public `pkg/tuya` client, README, auth example, and user guides are Tuya-focused. They do not include a consuming application adapter or rollout plan.

2. **PASS.** README documents supported operations, authentication, error inspection, options/injection, token ownership, and session close. The authentication, device-list/control, event, RTC/provider, upgrading, and CLI MDX guides match the current API. Provider-documented and implementation-derived behavior are distinguished in the API reference and guide copy.

3. **PASS.** README has Go version, CI, coverage, release, Go Reference, license, and docs badges. The badge destinations are live; coverage JSON/HTML and Pages root returned 200, and the latest release page resolved to v0.3.4. The new PR’s Pages artifact is built but not deployed yet, as expected.

4. **FAIL — blocking.** The schemas and generated outputs are broadly populated and drift checks pass, but the source gates do not enforce several explicit negative controls:
   - **Generated struct later writes bypass fixed-value validation.** In `tools/wiremodels/wire_construction.go:446-470`, a selector assignment has no key returned by `wireAssignmentParts` and is skipped by the mutation scanner. I added a compile-valid temp probe using generated `wire.RTCOfferBody.Type` (schema enum, only `offer`): later direct assignment, pointer-alias assignment, and package-global assignment to unregistered strings all passed the exact root `make wire-routes` command after temp inventory regeneration. `go test ./pkg/tuya` compiled the probe. The dynamic caller-input positive also remained accepted.
   - **Named-result generated map escapes to and is mutated by an unverified helper.** A compile-valid temp probe returned `wireRequestMap(wire.EncryptedRequestQuery{Encdata: value})` through a named map result, then passed it to a sibling sink that wrote `fields["review-unregistered-query"] = "fixed"`. After regenerating only the temp inventory, root `make wire-routes` passed. This is the exact kind of named-result escape the checklist requires to reject.
   - **Unresolved imported helper provenance fails open.** A helper in a separate package returning an unregistered fixed value, used as `Encdata: helper.Value()`, compiled and passed root `make wire-routes`. The gate must not infer caller input when it cannot prove the imported helper’s provenance.
   - **HTTP route can change after the schema guard.** `pkg/dependencies/httptransport/http.go:50-64` checks `IsKnownOperation(operation.Method, path)`, then uses `path` again in `requestURL`. I inserted a gofmt-valid `path += operation.Path` after the guard and before `requestURL`; `go test -run '^$' ./pkg/dependencies/httptransport` and root `make wire-routes` both passed. The gate checks route-construction/guard ordering but not that the validated path remains unchanged through send.
   - **Signature preimage string template is not schema-owned.** `pkg/tuya/encryption.go:31,561-597` embeds the `'||'` separator and `key=value` canonicalization used to form the X-sign preimage. `api/models/encryption.yaml` only models the X-sign header as a string; I found no schema component, generated declaration, or inventory entry for this library-owned format. No cookie transport was found (N/A).

   Other negative controls I probed did behave as intended: sibling function/method fixed returns, named-result bare returns, returned callback values, a 40-helper chain, recursive fixed fallback, and a global fixed initializer were rejected. The successful fail-open probes were removed; after cleanup, the temp clone’s `make wire-routes` passes and has no source diff.

5. **PASS.** Exact frozen-head CI succeeded, including a reviewer-independent exact-commit check. `.golangci.yml` sets literal `linters.default: all`; CI pins v2.14.0 and blocks on the full repository, CLI, and example runs. Exceptions are narrowed by path, linter, and reason. Local `make lint` and `make check` also passed. There are upstream linter deprecation warnings, but no lint issues or failed gate.

6. **PASS.** Deterministic synthetic HTTP, error, session, and MQTT paired transcripts exist; no private account capture was committed. Combined non-generated coverage is 83.2%, above the enforced 80% minimum. The remaining gap to the stated 90% target is reported rather than hidden.

7. **PASS.** Public API is under `pkg/tuya`; generated provider models are under `pkg/dependencymodels`; transport code is under `pkg/dependencies`. Responsibility schemas are split across authentication, common, devices, encryption, events, homes, message_queue, and RTC; public semantic projections use a separate schema. No catch-all wire/model bucket was found. The isolated consumer-module compile passed. Published-proxy verification remains a release gate under item 16.

8. **PASS.** `NewClient(...Option)` validates the required client ID, endpoints, nil/conflicting HTTP options, and MQTT factory. Defaults and override options are explicit. Reusable client options carry endpoint/transport config, not account tokens.

9. **PASS.** Account tokens and stateful message queue/RTC lifecycles are session-scoped. `Session.Tokens` and `SetTokens` expose caller-managed state, `Session.Close` stops its queue, and RTC streams have explicit stop/teardown methods. Client config can serve multiple accounts.

10. **PASS.** HTTP clients or RoundTrippers are injectable; MQTT creation is injectable through `WithMQTTClientFactory` and Paho `ClientOptions.SetCustomOpenConnectionFn`; RTC signaling can be replaced. No additional SDK-owned socket edge was found. The actual pinned Paho MQTT v1.5.1 framed exchange is replayed through `net.Pipe`, including success and denied-CONNACK paths, packet ordering, disconnect, and EOF.

11. **PASS.** Token refresh is an explicit `AuthService.RefreshToken` call returning rotated credentials. It does not mutate session tokens; docs and tests require callers to persist and explicitly call `Session.SetTokens`. Requests do not refresh implicitly.

12. **OPEN.** The exact PR artifact builds all guides and the generated reference; 801 internal links pass, and schema externalDocs were manually checked. However, published GitHub release notes for v0.2.0 and v0.3.1–v0.3.4 still link to `https://portpowered.github.io/go-tuya/docs/guides/provider-evidence/`. That route is absent from the exact artifact and the live URL serves a not-found fallback despite HTTP 200. The Docs workflow does not inspect GitHub release notes, so its success does not catch this regression. Preserve a valid redirect/alias or update the published release-note links, then recheck after Pages deployment. The PR Docs run intentionally skipped deployment; the final main Pages deployment still needs verification.

13. **OPEN.** README is compact and focused on install, short authenticated usage, capabilities, options, lifecycle, errors, and guide links. Other tracked docs are separated by audience; provenance and migration history stay in contributor/release material. Two concrete copy/navigation issues remain: the broken repeated release-note `provider-evidence` destination above, and tracked `TASKS.md` is an unlinked one-line file containing only `### NOtes` (no references point to it). Delete or give that file a clear current purpose, and repair the release-note destination. I did not inspect `docs/independent-review.md` per task direction.

14. **OPEN.** Findings in item 4 and items 12/13 remain unresolved. This Reviewer 2 report is saved in TEMP, not attached to the current repository review record, and I did not read the other current review report. Do not check item 14 or treat previous signoffs as resolving these findings. Both reviewers must verify fixes at the final implementation commit and add their separate item-by-item evidence to the repository’s single current review document.

15. **PASS.** HTTP and MQTT replay tests pair requests/frames with their exact response/frames and assert ordering, mismatch rejection, volatile-value rules, and cleanup. Authentication tests cover the QR/token exchanges; CLI tests cover auth errors and event cancellation/cleanup. No OAuth callback/PKCE or CSRF/OTP flow exists in this SDK, so those flow-specific requirements are N/A; the implemented QR/token flow is explicitly exercised. MQTT teardown tests require EOF rather than treating a timeout as proof of cleanup.

16. **OPEN — publication proof pending.** The standalone CLI is in `cmd/go-tuya`, consumes the public SDK, has help, JSON output, nonzero failures, cancellation, token-file/env/stdin paths, explicit export, explicit device-change commands, and offline paired CLI tests. CI runs pinned all-linter, build, test, vet, and module checks. The first CLI module release has not been published, and SDK v0.4.0 is not yet available from the public proxy. `.github/workflows/release-cli.yml` deliberately requires SDK v0.4.0 and rejects the development-only local `replace`; therefore a clean published CLI consumer install and `go install` proof cannot exist yet. SDK and CLI tags plus a main-branch Pages deployment remain post-merge publication gates.

## Merge and release disposition

**Do not merge at this commit.** The P1 source-gate bypasses in item 4 need fixes plus compile-valid regression tests run through the root CI/Makefile command; repair item 12/13’s release-note destination and remove or repurpose `TASKS.md`; then rerun exact CI and obtain two independent final-commit reviews. After those changes and reviews, implementation merge eligibility can be assessed before publishing. Final v0.4.0 SDK proxy verification, first CLI tag/consumer install, and main Pages deployment are separate post-merge publication evidence and remain pending.
