package tuya

import (
	"context"
	"fmt"
	"sync"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// RTCStream represents an active WebRTC stream with a Tuya camera device.
// Tuya cameras use cloud API-based signaling for WebRTC session establishment,
// following the protocol documented in https://github.com/AlexxIT/go2rtc/pull/1730.
type RTCStream struct {
	sessionID string
	deviceID  string
	sdpAnswer string
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	isAlive   bool
}

// RTCSessionInfo contains the session identifier and SDP answer returned by
// the configured RTC signaling client.
type RTCSessionInfo struct {
	SessionID string
	SDPAnswer string
}

// RTCSignaling defines the network edge used to exchange camera session offers
// and answers. The default implementation uses the Tuya encrypted HTTP API;
// callers can supply a WebSocket or other signaling implementation.
type RTCSignaling interface {
	Start(context.Context, StartRTCStreamRequest) (RTCSessionInfo, error)
	Stop(context.Context, StopRTCStreamRequest) error
}

type encryptedRTCSignaling struct {
	client *EncryptedClient
}

type encryptedRTCSessionResponse struct {
	SessionID string `json:"session_id"`
	SDP       string `json:"sdp"`
}

func (s encryptedRTCSignaling) Start(ctx context.Context, req StartRTCStreamRequest) (RTCSessionInfo, error) {
	resp, err := s.client.Post(ctx,
		fmt.Sprintf(wire.RouteStartRTCSession, req.DeviceID),
		nil,
		map[string]interface{}{"sdp": req.SDPOffer, "type": "offer"},
		&req,
	)
	if err != nil {
		return RTCSessionInfo{}, fmt.Errorf("failed to initiate WebRTC session: %w", err)
	}

	response, err := serialize[encryptedRTCSessionResponse](resp)
	if err != nil {
		return RTCSessionInfo{}, fmt.Errorf("failed to parse WebRTC session response: %w", err)
	}
	return RTCSessionInfo{SessionID: response.SessionID, SDPAnswer: response.SDP}, nil
}

func (s encryptedRTCSignaling) Stop(ctx context.Context, req StopRTCStreamRequest) error {
	_, err := s.client.Delete(ctx,
		fmt.Sprintf(wire.RouteStopRTCSession, req.DeviceID, req.SessionID),
		nil,
		&req,
	)
	return err
}

// StartRTCStream initiates a WebRTC session with a Tuya camera device.
// It sends the SDP offer to the Tuya cloud API and returns an RTCStream
// containing the SDP answer for the caller to complete the WebRTC handshake.
func (c *Session) StartRTCStream(ctx context.Context, req StartRTCStreamRequest) (*RTCStream, error) {
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device ID is required")
	}
	if req.SDPOffer == "" {
		return nil, fmt.Errorf("SDP offer is required")
	}

	signaling := c.rtcSignaling
	if signaling == nil && c.EncryptedClient != nil {
		signaling = encryptedRTCSignaling{client: c.EncryptedClient}
	}
	if signaling == nil {
		return nil, fmt.Errorf("RTC signaling client is not configured")
	}
	info, err := signaling.Start(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate WebRTC session: %w", err)
	}

	if info.SDPAnswer == "" {
		return nil, fmt.Errorf("empty SDP answer received from Tuya API")
	}

	streamCtx, cancel := context.WithCancel(ctx)

	return &RTCStream{
		sessionID: info.SessionID,
		deviceID:  req.DeviceID,
		sdpAnswer: info.SDPAnswer,
		ctx:       streamCtx,
		cancel:    cancel,
		isAlive:   true,
	}, nil
}

// StopRTCStream stops a WebRTC stream by session ID via the Tuya cloud API.
func (c *Session) StopRTCStream(ctx context.Context, req StopRTCStreamRequest) error {
	signaling := c.rtcSignaling
	if signaling == nil && c.EncryptedClient != nil {
		signaling = encryptedRTCSignaling{client: c.EncryptedClient}
	}
	if signaling == nil {
		return fmt.Errorf("RTC signaling client is not configured")
	}
	err := signaling.Stop(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to stop WebRTC session: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the RTC stream and cancels its context.
func (s *RTCStream) Stop() error {
	s.mu.Lock()
	if !s.isAlive {
		s.mu.Unlock()
		return nil
	}
	s.isAlive = false
	s.mu.Unlock()

	s.cancel()
	return nil
}

// GetSessionID returns the session ID assigned by the Tuya cloud.
func (s *RTCStream) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

// GetDeviceID returns the device ID this stream is connected to.
func (s *RTCStream) GetDeviceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceID
}

// GetSDPAnswer returns the SDP answer from the Tuya camera.
func (s *RTCStream) GetSDPAnswer() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sdpAnswer
}

// IsAlive returns whether the stream is still active.
func (s *RTCStream) IsAlive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isAlive
}
