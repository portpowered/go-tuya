// Package mqtttransport isolates the pinned Paho client boundary used by the SDK.
package mqtttransport

import paho "github.com/eclipse/paho.mqtt.golang"

// Client is Paho's MQTT client interface used by the public session adapter.
type Client = paho.Client

// ClientOptions preserves the Paho connection hooks accepted by the SDK.
type ClientOptions = paho.ClientOptions

// Message is a received Paho MQTT packet payload and topic.
type Message = paho.Message

// MessageHandler receives a decoded Paho message.
type MessageHandler = paho.MessageHandler

// Token is the completion handle returned by Paho operations.
type Token = paho.Token

// NewClientOptions creates options for the pinned Paho transport.
func NewClientOptions() *ClientOptions {
	return paho.NewClientOptions()
}

// NewClient opens the pinned Paho MQTT client.
func NewClient(options *ClientOptions) Client { //nolint:ireturn // Paho exposes the client interface as its construction contract.
	return paho.NewClient(options)
}
