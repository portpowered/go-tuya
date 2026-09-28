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
