package tuya //nolint:testpackage // Reuses the paired encrypted-config matcher and checks queue teardown state.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const mqttFramedTimeout = 10 * time.Second
const mqttFixtureServer = "server"
const mqttFixtureAccessToken = "synthetic-access-token"
const mqttFixtureRefreshToken = "synthetic-refresh-token"

var errMQTTFramedMismatch = errors.New("MQTT framed replay mismatch")

type mqttBinaryFrame struct {
	Direction string `json:"direction"`
	Hex       string `json:"hex"`
	PacketID  bool   `json:"packet_id"`
}

type mqttBinaryTranscript struct {
	Provenance string            `json:"provenance"`
	Frames     []mqttBinaryFrame `json:"frames"`
}

func loadMQTTBinaryTranscript(t *testing.T) mqttBinaryTranscript {
	t.Helper()

	return loadMQTTBinaryTranscriptFrom(t, "../../tests/replay/fixtures/mqtt/synthetic/paho-framed-session.synthetic.json")
}

func loadMQTTBinaryTranscriptFrom(t *testing.T, path string) mqttBinaryTranscript {
	t.Helper()

	// #nosec G304 -- callers supply only the two fixed checked-in synthetic MQTT transcript paths.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var transcript mqttBinaryTranscript

	err = json.Unmarshal(data, &transcript)
	if err != nil {
		t.Fatal(err)
	}

	if transcript.Provenance == "" || len(transcript.Frames) == 0 {
		t.Fatal("missing synthetic MQTT frame provenance or exchanges")
	}

	return transcript
}

// Read the original packet bytes, including its fixed header and length encoding.
func readMQTTBinaryPacket(connection net.Conn) ([]byte, int, error) {
	packet := make([]byte, 0, 5)
	packet = append(packet, 0)

	_, err := io.ReadFull(connection, packet)
	if err != nil {
		return nil, 0, fmt.Errorf("read MQTT packet header: %w", err)
	}

	remaining := 0
	multiplier := 1

	for range 4 {
		var digit [1]byte

		_, err = io.ReadFull(connection, digit[:])
		if err != nil {
			return nil, 0, fmt.Errorf("read MQTT remaining length: %w", err)
		}

		packet = append(packet, digit[0])
		remaining += int(digit[0]&0x7f) * multiplier
		multiplier *= 128

		if digit[0]&0x80 == 0 {
			headerLength := len(packet)
			body := make([]byte, remaining)

			_, err = io.ReadFull(connection, body)
			if err != nil {
				return nil, 0, fmt.Errorf("read MQTT packet body: %w", err)
			}

			return append(packet, body...), headerLength, nil
		}
	}

	return nil, 0, errMQTTFramedMismatch
}

func matchMQTTBinaryPacket(actual, expected []byte, headerLength int, bindID bool, seen map[uint16]bool) (uint16, error) {
	if len(actual) != len(expected) {
		return 0, errMQTTFramedMismatch
	}

	var identifier uint16

	if bindID {
		if headerLength+2 > len(actual) || (actual[0] != 0x82 && actual[0] != 0xa2) {
			return 0, errMQTTFramedMismatch
		}

		identifier = binary.BigEndian.Uint16(actual[headerLength : headerLength+2])
		if identifier == 0 || seen[identifier] {
			return 0, errMQTTFramedMismatch
		}

		actual = bytes.Clone(actual)
		copy(actual[headerLength:headerLength+2], expected[headerLength:headerLength+2])
	}

	if !bytes.Equal(actual, expected) {
		return 0, errMQTTFramedMismatch
	}

	if bindID {
		seen[identifier] = true
	}

	return identifier, nil
}

func replayMQTTBinaryFrame(connection net.Conn, frame mqttBinaryFrame, seen map[uint16]bool, identifier uint16) (uint16, error) {
	expected, err := hex.DecodeString(frame.Hex)
	if err != nil {
		return 0, fmt.Errorf("decode MQTT fixture frame: %w", err)
	}

	switch frame.Direction {
	case mqttFixtureClient:
		actual, headerLength, readErr := readMQTTBinaryPacket(connection)
		if readErr != nil {
			return 0, readErr
		}

		return matchMQTTBinaryPacket(actual, expected, headerLength, frame.PacketID, seen)
	case mqttFixtureServer:
		if frame.PacketID {
			err = bindMQTTBinaryAcknowledgement(expected, identifier)
			if err != nil {
				return 0, err
			}
		}

		_, err = connection.Write(expected)
		if err != nil {
			return 0, fmt.Errorf("write MQTT fixture frame: %w", err)
		}

		return identifier, nil
	default:
		return 0, errMQTTFramedMismatch
	}
}

func replayMQTTBinaryFrames(connection net.Conn, transcript mqttBinaryTranscript) error {
	seen := make(map[uint16]bool)

	var identifier uint16

	for index, frame := range transcript.Frames {
		var err error

		identifier, err = replayMQTTBinaryFrame(connection, frame, seen, identifier)
		if err != nil {
			return fmt.Errorf("MQTT fixture frame %d: %w", index, err)
		}
	}
	// MQTT 3.1.1 DISCONNECT has no acknowledgement. Require actual EOF after it.
	var extra [1]byte

	count, err := connection.Read(extra[:])
	if count != 0 || !errors.Is(err, io.EOF) {
		return fmt.Errorf("MQTT teardown did not close after DISCONNECT: %w", errMQTTFramedMismatch)
	}

	return nil
}

func newMQTTFramedConnections(t *testing.T, transcript mqttBinaryTranscript) (net.Conn, <-chan error) {
	t.Helper()

	clientConnection, brokerConnection := net.Pipe()

	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = brokerConnection.Close()
	})

	err := brokerConnection.SetDeadline(time.Now().Add(mqttFramedTimeout))
	if err != nil {
		t.Fatal(err)
	}

	brokerDone := make(chan error, 1)

	go func() { brokerDone <- replayMQTTBinaryFrames(brokerConnection, transcript) }()

	return clientConnection, brokerDone
}

func newMQTTFramedSession(t *testing.T, httpReplay *mqttHTTPReplay, clientConnection net.Conn) *Session {
	t.Helper()

	client, err := newSyntheticClient(
		WithHTTPTransport(httpReplay), WithClientID("synthetic-client-id"), WithCloudAPIURL("https://api.example.invalid"),
		WithMQTTClientFactory(func(options *mqtt.ClientOptions) mqtt.Client {
			options.SetCustomOpenConnectionFn(func(uri *url.URL, actual mqtt.ClientOptions) (net.Conn, error) {
				if uri.String() != "ssl://mqtt.example.invalid:8883" || actual.ClientID != syntheticMQTTClientID ||
					actual.Username != syntheticMQTTUsername || actual.Password != syntheticMQTTPassword {
					return nil, errMQTTFramedMismatch
				}

				return clientConnection, nil
			})

			return mqtt.NewClient(options)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	var tokens Tokens

	tokens.AccessToken = mqttFixtureAccessToken
	tokens.RefreshToken = mqttFixtureRefreshToken
	tokens.ExpireTime = time.Now().Add(time.Hour).UnixMilli()
	session := client.NewSession(tokens)

	t.Cleanup(func() { _ = session.Close(context.Background()) })

	return session
}

func registerMQTTFramedListeners(ctx context.Context, t *testing.T, queue *SharingMessageQueueImpl, events chan<- string) {
	t.Helper()

	var ownerRequest AddMessageListenerRequest

	ownerRequest.Topic = mqttFixtureGeneral
	ownerRequest.Callback = func(topic string, event any) {
		value, valid := event.(Event)
		if valid {
			events <- topic + ":" + value.GetDeviceID()
		}
	}

	_, err := queue.AddMessageListener(ctx, ownerRequest)
	if err != nil {
		t.Fatal(err)
	}

	var deviceRequest AddDeviceListenerRequest

	deviceRequest.DeviceID = mqttFixtureDevice1
	deviceRequest.Callback = func(deviceID string, event Event) { events <- deviceID + ":" + event.GetDeviceID() }

	_, err = queue.AddDeviceListener(ctx, deviceRequest)
	if err != nil {
		t.Fatal(err)
	}
}

func assertMQTTFramedEvents(ctx context.Context, t *testing.T, events <-chan string) {
	t.Helper()

	for _, want := range []string{"general:device-1", "device-1:device-1"} {
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("MQTT event = %q, want %q", got, want)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}

func stopMQTTFramedSession(
	ctx context.Context, t *testing.T, session *Session, brokerDone <-chan error,
	httpTranscript *mqttReplayTranscript, httpReplay *mqttHTTPReplay,
) {
	t.Helper()

	queue := session.MessageQueue

	var removeRequest RemoveDeviceListenerRequest

	removeRequest.DeviceID = mqttFixtureDevice1

	_, err := queue.RemoveDeviceListener(ctx, removeRequest)
	if err != nil {
		t.Fatal(err)
	}

	assertMQTTFramedSessionClosed(ctx, t, session, brokerDone, httpTranscript, httpReplay)
}

func assertMQTTFramedSessionClosed(
	ctx context.Context, t *testing.T, session *Session, brokerDone <-chan error,
	httpTranscript *mqttReplayTranscript, httpReplay *mqttHTTPReplay,
) {
	t.Helper()

	err := session.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case err = <-brokerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	err = httpTranscript.verifyConsumed()

	status := session.MessageQueue.Status()
	if err != nil || !httpReplay.consumed || status.Running || status.Connected {
		t.Fatalf("unconsumed exchange or active queue after teardown: %v", err)
	}
}

func TestPahoMQTTFramedPairedReplay(t *testing.T) {
	t.Parallel()
	transcript := loadMQTTBinaryTranscript(t)
	httpTranscript, configPair := loadSyntheticMQTTReplay(t)
	httpTranscript.Frames = httpTranscript.Frames[:2]
	httpReplay := &mqttHTTPReplay{transcript: &httpTranscript, pair: configPair, consumed: false}
	connection, brokerDone := newMQTTFramedConnections(t, transcript)
	session := newMQTTFramedSession(t, httpReplay, connection)

	ctx, cancel := context.WithTimeout(context.Background(), mqttFramedTimeout)
	defer cancel()

	events := make(chan string, 2)
	registerMQTTFramedListeners(ctx, t, session.MessageQueue, events)

	var startRequest MessageQueueStartRequest

	_, err := session.MessageQueue.Start(ctx, startRequest)
	if err != nil {
		t.Fatal(err)
	}

	assertMQTTFramedEvents(ctx, t, events)
	stopMQTTFramedSession(ctx, t, session, brokerDone, &httpTranscript, httpReplay)
}

func TestPahoMQTTFramedDeniedConnectReplay(t *testing.T) {
	t.Parallel()

	transcript := loadMQTTBinaryTranscriptFrom(t, "../../tests/replay/fixtures/mqtt/synthetic/paho-framed-denied.synthetic.json")
	httpTranscript, configPair := loadSyntheticMQTTReplay(t)
	httpTranscript.Frames = httpTranscript.Frames[:2]
	httpReplay := &mqttHTTPReplay{transcript: &httpTranscript, pair: configPair, consumed: false}
	connection, brokerDone := newMQTTFramedConnections(t, transcript)
	session := newMQTTFramedSession(t, httpReplay, connection)

	ctx, cancel := context.WithTimeout(context.Background(), mqttFramedTimeout)
	defer cancel()

	var startRequest MessageQueueStartRequest

	response, err := session.MessageQueue.Start(ctx, startRequest)
	if err == nil || response.Success {
		t.Fatalf("denied CONNECT response = %+v, error = %v; want typed start failure", response, err)
	}

	var clientErr *ClientError

	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorTransport {
		t.Fatalf("denied CONNECT error = %v, want typed transport error", err)
	}

	assertMQTTFramedSessionClosed(ctx, t, session, brokerDone, &httpTranscript, httpReplay)
}

func TestMQTTBinaryMatcherRejectsMutations(t *testing.T) {
	t.Parallel()

	transcript := loadMQTTBinaryTranscript(t)
	for _, frame := range transcript.Frames {
		if frame.Direction != mqttFixtureClient {
			continue
		}

		expected, err := hex.DecodeString(frame.Hex)
		if err != nil {
			t.Fatal(err)
		}

		for offset := range expected {
			if frame.PacketID && (offset == 2 || offset == 3) {
				continue
			}

			mutated := bytes.Clone(expected)
			mutated[offset] ^= 1

			_, err = matchMQTTBinaryPacket(mutated, expected, 2, frame.PacketID, make(map[uint16]bool))
			if err == nil {
				t.Fatalf("MQTT frame byte %d mutation accepted", offset)
			}
		}

		assertMQTTBinaryIdentifierNegatives(t, expected, frame.PacketID)
	}
}

func assertMQTTBinaryIdentifierNegatives(t *testing.T, expected []byte, bindID bool) {
	t.Helper()

	if bindID {
		zero := bytes.Clone(expected)
		zero[2], zero[3] = 0, 0

		_, err := matchMQTTBinaryPacket(zero, expected, 2, true, make(map[uint16]bool))
		if err == nil {
			t.Fatal("zero packet identifier accepted")
		}

		_, err = matchMQTTBinaryPacket(expected, expected, 2, true, map[uint16]bool{1: true})
		if err == nil {
			t.Fatal("duplicate packet identifier accepted")
		}
	}
}

func bindMQTTBinaryAcknowledgement(packet []byte, identifier uint16) error {
	if identifier == 0 || len(packet) < 4 || (packet[0] != 0x90 && packet[0] != 0xb0) {
		return errMQTTFramedMismatch
	}

	binary.BigEndian.PutUint16(packet[2:4], identifier)

	return nil
}
