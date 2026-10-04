# MQTT transcript provenance

`owner-device-session.synthetic.json` is a hand-authored synthetic transcript.
It starts with the paired encrypted HTTP configuration exchange from
`device-sharing/synthetic/remaining-operations.synthetic.json`, then follows
the real queue connection path through the injected MQTT factory and client.
It pairs ordered connect, subscribe, unsubscribe, and disconnect actions with
broker message frames and public callbacks for one owner topic and one device
status topic. It contains no broker or customer account capture. The local
topic template is currently used only for address formatting, so there is no
local-topic subscription exchange to replay.

## Paho framed connection replay

`paho-framed-session.synthetic.json` contains hand-authored MQTT 3.1.1 packet
bytes for Paho v1.5.1. It is synthetic and contains no account or broker capture.
The test runs the real Paho client over an injected `net.Pipe` connection and
pairs CONNECT/CONNACK, SUBSCRIBE/SUBACK, event PUBLISH, and
UNSUBSCRIBE/UNSUBACK before DISCONNECT and actual socket EOF.
Every request byte is compared before its paired response is written. Only
packet identifiers vary: they must be nonzero and distinct and are echoed into
the paired acknowledgement. Negative tests mutate every fixed request byte and
reject zero or duplicate identifiers. EOF is required after DISCONNECT; a read
timeout does not prove cleanup.
