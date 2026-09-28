# MQTT transcript provenance

`owner-device-session.synthetic.json` is a hand-authored synthetic transcript.
It pairs ordered client connect, subscribe, unsubscribe, and disconnect actions
with broker message frames and public callbacks for one owner topic and one
device status topic. It contains no broker or customer account capture. The
local-topic template is currently used only for address formatting, so there
is no local-topic subscription exchange to replay.
