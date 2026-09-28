# Independent paired-replay review

Reviewed commit: `c92663521c7cd28b1ce7bdea552221c1f3e6aee4`.
Standard: item 15 of `go-third-party-template/docs/library-standards.md`, with
the fixture-provenance detail in its `docs/verification.md`. This review was
performed independently of the implementation. The earlier 14-item report
predates item 15.

| Item | Verdict | Evidence |
| --- | --- | --- |
| 15. Paired replay | **Open** | Seven stored synthetic HTTP pairs cover six distinct successful operations and one error outcome. The OpenAPI inventory has 28 HTTP operations; 22 have no stored pair. MQTT has no stored ordered bidirectional transcript. See findings below. |
| 14. Independent verification | **Open** | The previous signoff cannot carry forward while item 15 is open. Recheck all checklist items at the eventual final commit. |

## Findings

1. **Incomplete HTTP inventory.** `api/openapi.yaml` has 28 `operationId`
   entries. `tests/replay/fixtures/` has success pairs for
   `generateLoginQRCode`, `validateLoginCode`, `queryHomes`,
   `queryHomeDevices`, `queryDeviceStatus`, and `getDeviceList`, plus a
   `queryHomes` API-error pair. The remaining 22 operations have no stored
   request/response pair. `pkg/tuya/route_synthetic_test.go` exercises many
   routes with an inline `httptest` handler, but checks mostly method, decoded
   path, and two headers, then constructs a response without an exchange
   fixture. That test does not supply the request multimap, escaped path,
   headers, body, and paired response required by item 15.
2. **No MQTT transcript.** `pkg/tuya/messages_synthetic_test.go` uses an
   in-memory MQTT client that collects subscription topic strings and invokes
   inbound message handlers with inline payloads. It has no stored ordered
   client/server transcript for owner and device channels, including connect,
   subscribe, event delivery, unsubscribe, and disconnect behavior. It does
   not reject an unexpected or duplicate frame or assert exhaustion of a
   transcript. The local-topic channel is currently used for address
   formatting, not a subscribed transport exchange.
3. **Volatile values are weakly matched.** The seven HTTP fixtures use
   `<nonempty>` for `X-requestId` and `X-sign`. This accepts arbitrary bytes
   instead of checking the identifier/signature format or signature meaning.
   `<unix-millis>` accepts any signed integer, including negative values;
   `<encrypted>` checks only base64 decoding and a 12-byte minimum. The
   custom validators decrypt `encdata` for `queryHomeDevices` and
   `getDeviceList`, which is stronger, but other signed requests have no
   corresponding signature validation. Item 15 requires explicit match rules
   that validate volatile values' format or decoded meaning.
4. **Transport safeguards need exhaustive negative coverage.** In
   `tests/replay/replay_test.go`, `fixtureTransport.RoundTrip` correctly
   matches a stored request before returning its response, rejects unknown
   method/path keys and repeated calls, and the replay tests compare their
   observed call order with explicit expected keys. The one negative test
   actually exercises a query mismatch and duplicate call. It does not
   exercise origin, escaped path, header, body, or unexpected-call failure,
   so regressions in those rejection paths would not be caught. An explicit
   assertion that all configured exchanges were consumed would also make the
   fixture suite robust as cases are added; present tests enumerate expected
   calls by hand.

The existing fixture files are correctly marked synthetic and their
provenance notes do not claim account capture. `go test -count=1 -race ./tests/replay`
passed at the reviewed commit. Passing this subset does not close the
inventory and transcript gaps above.

**Signoff:** neither item 15 nor renewed item 14 can be checked at this
commit.
