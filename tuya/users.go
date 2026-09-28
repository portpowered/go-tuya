package tuya

import "context"

// UserService provides methods for managing user sessions
type UserService service

// Unload disables a current user session, when the user has logged out.
func (s *UserService) Unload(_ context.Context, _ string) (UnloadResponse, error) {
	// TODO: implement unload
	return UnloadResponse{}, nil
}

// UnloadRequest represents a request to unload a user session
type UnloadRequest struct {
	TerminalID string `json:"terminal_id"`
}

// UnloadResponse represents the response from unloading a user session
type UnloadResponse struct {
}
