# Independent paired-replay review

Initial reviewed commit: `c92663521c7cd28b1ce7bdea552221c1f3e6aee4`.
Re-audited implementation commit: `8475077fa4178c7111771a2422055ac5fa7805b7`.
Final item-15 implementation re-audit: `efde7aef0988ef3a068cf383aaca29e318bece0c`.
Standard: item 15 of `go-third-party-template/docs/library-standards.md`, with
the fixture-provenance detail in its `docs/verification.md`. This review was
performed independently of the implementation. The earlier 14-item report
predates item 15.

| Item | Verdict | Evidence |
| --- | --- | --- |
| 15. Paired replay | **Verified at `efde7ae`** | All 28 HTTP operations have consumed synthetic pairs. The MQTT transcript now runs through the real queue startup, paired config HTTP exchange, injected factory and `Connect`, then ordered subscriptions, events, callbacks, unsubscribe, and disconnect. |
| 14. Independent verification | **Open** | The `v0.3.1` exact-tag release predates item 15. Recheck all checklist items and the release gates on the final tagged commit before renewing signoff. |

## Initial findings at `c926635`

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

## Re-audit at `8475077`

| Initial finding | Disposition and independent evidence |
| --- | --- |
| HTTP inventory | **Resolved.** The six original successful operations plus 22 stored success pairs in `remaining-operations.synthetic.json` cover all 28 OpenAPI operation IDs. `TestHTTPReplaySchemaInventoryAndExhaustion` fails on a missing or duplicate operation ID, and `TestRemainingOperationsPairedReplay` invokes the 22 methods in fixture order and checks consumption. The separate API-error pair remains. |
| MQTT transcript | **Partly resolved.** `owner-device-session.synthetic.json` now orders connect, owner/device subscriptions, inbound protocol-4 messages, callbacks, unsubscribe, and disconnect. `mqttReplayTranscript` rejects a wrong or duplicate frame and fails on unconsumed frames. The connection bypass described below remains. |
| Volatile values | **Resolved for the HTTP pairs.** `matchFixtureRequest` checks UUID request IDs, positive recent millisecond timestamps, decrypted query/body meaning, and a recomputed HMAC signature; fixed synthetic credentials match exactly. Negative tests tamper with each of those fields. |
| Transport safeguards | **Resolved for the HTTP pairs.** The original transport now checks exhaustion. Its negative test covers origin, escaped path, header, body, unexpected and duplicate calls. The new ordered transport refuses a mismatch before serving the response, rejects requests after exhaustion, and asserts all 22 pairs were consumed. |

**Remaining MQTT gap:** `pkg/tuya/mqtt_replay_test.go` replaces
`queue.State.connect` with a closure that itself appends the expected
`connect` frame, installs a hard-coded `mqConfig` and mock broker, and calls
`onConnect`. The test therefore never executes `mqttState.connectMQTT` in
`pkg/tuya/messages.go`. It does not match the MQTT client options derived
from the paired `getMessageQueueConfig` HTTP result (broker URL, client ID,
username, password, handlers), the factory call, actual `mqtt.Client.Connect`,
or its completion/error against a transcript step. The separate
`TestMQTTClientFactoryCanBeReplaced` checks only client ID via an inline
`httptest` response and does not store or consume that MQTT exchange. Thus
the transcript verifies subscription and delivery behavior after a mock
connection, while the supported MQTT connection edge remains outside paired
replay. The owner/device message data are correctly labeled synthetic; the
local-topic template is not subscribed.

At `8475077`, `make replay`, `make lint`, `make check`, and `make coverage`
passed; the non-generated package coverage was 82.4% (1,304/1,583). Fresh
`-count=1 -race` runs of the HTTP inventory, matcher-negative, and MQTT
transcript tests passed. These results do not close the connection-edge gap.

**At `8475077`:** item 15 and renewed item 14 remained unchecked. Once MQTT
establishment is replayed through the actual injected factory/`Connect`
path, independently recheck the fix and every checklist item at the final
commit.

## Final item-15 re-audit at `efde7ae`

The remaining MQTT finding is **resolved**. The revised
`TestSyntheticMQTTPairedTranscript` calls `MessageQueue.Start` without
overriding `state.connect`, so it executes `connectMQTT`. Its injected HTTP
transport matches the stored `getMessageQueueConfig` pair before returning
the response. It validates method, origin, escaped path, empty query,
relevant headers, decrypted request body, and recomputed signature. The
MQTT factory checks the broker URL, client ID, username, password, and
handlers derived from that response. Its client `Connect` records the next
transcript step and invokes the installed on-connect handler. The transcript
then consumes owner/device subscriptions, two inbound messages and their
public callbacks, unsubscribe, and disconnect in order. Wrong, duplicate,
and unconsumed steps fail. The separate 28-operation HTTP inventory and
matcher-negative tests remain passing.

Fresh `-count=1 -race` runs of the MQTT transcript and HTTP replay/inventory
tests passed. `make replay`, `make lint`, `make check`, and `make coverage`
passed; non-generated package coverage was 82.6% (1,308/1,583).

**Item 15 signoff: verified at `efde7ae`.** Item 14 remains open because
the published `v0.3.1` exact-tag workflow predates the new replay suite.
The next tagged release must run the current generation, route/channel,
coverage, replay, race, compatibility, public-consumer and documentation
gates, followed by a full 15-item independent review at that final commit.

## Exact-tag recheck at `v0.3.3`

`v0.3.3` resolves to `2b598febb4f656e407e0f6a525c8633063def27b`.
The only code change after the item-15 review commit `0364688` was gofmt
formatting in `mqtt_replay_test.go`; all fixture, matcher, and MQTT startup
behavior is identical. The preceding `v0.3.2` tag passed Release but failed
main CI formatting. At `v0.3.3`, main CI
[`36495828460`](https://github.com/portpowered/go-tuya/actions/runs/36495828460),
Documentation [`36495828389`](https://github.com/portpowered/go-tuya/actions/runs/36495828389),
and exact-tag Release [`36495952279`](https://github.com/portpowered/go-tuya/actions/runs/36495952279)
all passed. The release replay step reran the paired HTTP and MQTT tests;
race, schema drift, route/channel, coverage, and public-proxy consumer gates
also passed. I reran `make lint`, `make check`, and `make replay` locally at
the formatted commit. **Item 15 remains verified.** The full
[15-item independent re-review](independent-verification.md) closes renewed
item 14 at this tag.
