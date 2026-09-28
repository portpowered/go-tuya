package main

import (
	"strings"
	"testing"
)

func TestParseRoutesAndGenerate(t *testing.T) {
	source := `openapi: 3.1.0
paths:
  /v1.0/devices/{device_id}:
    get:
      operationId: getDevice
    delete:
      operationId: deleteDevice
components:
  schemas: {}
`
	routes, err := parseRoutes(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(routes))
	}
	generated, err := generate(routes)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`RouteGetDevice`,
		`"/v1.0/devices/%s"`,
		`MethodDeleteDevice`,
		`"DELETE"`,
	} {
		if !strings.Contains(string(generated), expected) {
			t.Fatalf("generated routes missing %q", expected)
		}
	}
}

func TestParseChannelsAndGenerate(t *testing.T) {
	source := `asyncapi: 3.0.0
channels:
  deviceStatus:
    address: '{deviceTopic}/sta'
operations:
  receiveStatus:
    action: receive
`
	channels, err := parseChannels(source)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := generateMQTT(channels)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `MQTTChannelDeviceStatus`) ||
		!strings.Contains(string(generated), `"{deviceTopic}/sta"`) {
		t.Fatalf("generated channels missing deviceStatus: %s", generated)
	}
}

func TestGateRejectsUnknownPath(t *testing.T) {
	missing, err := missingPathLiterals("synthetic.go", []byte(`package tuya
func bad() { _ = "/v1.0/ghosts" }
`), map[string]bool{"/v1.0/devices": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "/v1.0/ghosts") {
		t.Fatalf("unknown path escaped gate: %v", missing)
	}
}

func TestGateRejectsChangedMethodAndUnknownChannel(t *testing.T) {
	operations := map[string]bool{"OperationGetDevice": true}
	channels := map[string]bool{"MQTTChannelDeviceStatus": true}
	cases := []struct {
		name   string
		source string
	}{
		{"handwritten method", `package tuya
func bad() { c.EncryptedClient.Post(ctx, "/v1.0/devices", nil, nil, req) }
`},
		{"unknown operation", `package tuya
func bad() { c.EncryptedClient.requestOperation(ctx, wire.OperationGhost(), nil, nil, nil, req) }
`},
		{"unknown channel", `package tuya
func bad() { subscribeChannel(client, wire.MQTTChannelGhost, "topic") }
`},
		{"direct subscription", `package tuya
func bad() { client.Subscribe("topic", 0, nil) }
`},
		{"direct unsubscription", `package tuya
func bad() { client.Unsubscribe("topic") }
`},
		{"mismatched direct method", `package tuya
func bad() { _ = wire.RouteGetDevice; http.NewRequestWithContext(ctx, wire.MethodDeleteDevice, url, nil) }
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateCallSites("synthetic.go", []byte(tc.source), operations, channels); err == nil {
				t.Fatal("invalid callsite escaped gate")
			}
		})
	}
}
