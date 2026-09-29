package replay_test

import (
	"errors"
	"fmt"
)

func testMismatchf(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), errReplayTestMismatch)
}

var (
	errReplayTestAddDeviceUserResult = errors.New("addDeviceUser fixture result is not a string")
	errReplayTestCommandFailed       = errors.New("command failed")
	errReplayTestDeleteFailed        = errors.New("delete failed")
	errReplayTestDeleteUserFailed    = errors.New("delete user failed")
	errReplayTestEncdataMissing      = errors.New("encdata query parameter is missing")
	errReplayTestRenameFailed        = errors.New("rename failed")
	errReplayTestRenameOutletFailed  = errors.New("rename outlet failed")
	errReplayTestRequestIDMissing    = errors.New("x-requestId is missing")
	errReplayTestMismatch            = errors.New("synthetic replay mismatch")
	errReplayTestInvalidFixture      = errors.New("invalid synthetic replay fixture")
	errReplayTestResetFailed         = errors.New("reset failed")
	errReplayTestSignMissing         = errors.New("x-sign is missing")
	errReplayTestSignature           = errors.New("x-sign does not authenticate request headers and encrypted data")
	errReplayTestUpdateUserFailed    = errors.New("update user failed")
)
