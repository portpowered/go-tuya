package tuya

import (
	"context"
	"fmt"
	"sync"
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
	client    *EncryptedClient
	authCtx   *AuthorizationContext
	mu        sync.RWMutex
	isAlive   bool
}

// webrtcSessionResponse represents the Tuya API response for WebRTC session initiation.
type webrtcSessionResponse struct {
	SessionID string `json:"session_id"`
	SDP       string `json:"sdp"`
}

// StartRTCStream initiates a WebRTC session with a Tuya camera device.
// It sends the SDP offer to the Tuya cloud API and returns an RTCStream
// containing the SDP answer for the caller to complete the WebRTC handshake.
func (c *ClientImpl) StartRTCStream(ctx context.Context, req StartRTCStreamRequest) (*RTCStream, error) {
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device ID is required")
	}
	if req.SDPOffer == "" {
		return nil, fmt.Errorf("SDP offer is required")
	}

	resp, err := c.EncryptedClient.Post(ctx,
		fmt.Sprintf("/v1.0/m/life/ipc/%s/webrtc/session", req.DeviceID),
		nil,
		map[string]interface{}{
			"sdp":  req.SDPOffer,
			"type": "offer",
		},
		&req,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate WebRTC session: %w", err)
	}

	sessionResp, err := serialize[webrtcSessionResponse](resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebRTC session response: %w", err)
	}

	if sessionResp.SDP == "" {
		return nil, fmt.Errorf("empty SDP answer received from Tuya API")
	}

	streamCtx, cancel := context.WithCancel(ctx)

	return &RTCStream{
		sessionID: sessionResp.SessionID,
		deviceID:  req.DeviceID,
		sdpAnswer: sessionResp.SDP,
		ctx:       streamCtx,
		cancel:    cancel,
		client:    c.EncryptedClient,
		authCtx:   req.AuthorizationContext,
		isAlive:   true,
	}, nil
}

// StopRTCStream stops a WebRTC stream by session ID via the Tuya cloud API.
func (c *ClientImpl) StopRTCStream(ctx context.Context, req StopRTCStreamRequest) error {
	_, err := c.EncryptedClient.Delete(ctx,
		fmt.Sprintf("/v1.0/m/life/ipc/%s/webrtc/session/%s", req.DeviceID, req.SessionID),
		nil,
		&req,
	)
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
