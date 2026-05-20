package ecspresso

import (
	"errors"
	"fmt"

	"github.com/aws/smithy-go"
)

// Sentinel errors. Use errors.Is to test for a specific category, and
// fmt.Errorf("...: %w", ..., Err...) to construct a wrapped instance
// that carries context. Detection by type (errors.As against the old
// string-typed errors) is no longer supported.
var (
	ErrSkipVerify       = errors.New("skip verify")
	ErrNotFound         = errors.New("not found")
	ErrConflictOptions  = errors.New("conflicting options")
	ErrPermissionDenied = errors.New("permission denied")
)

func isPermissionError(err error) bool {
	// Check if it's wrapped in OperationError
	var oe *smithy.OperationError
	if errors.As(err, &oe) {
		err = oe.Err
	}

	// Check the actual API error
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "AccessDeniedException", "UnauthorizedException",
			"Forbidden", "AccessDenied", "InvalidUserID.NotFound":
			return true
		}
	}
	return false
}

func wrapPermissionError(err error) error {
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		return fmt.Errorf("%s: %w", err.Error(), ErrPermissionDenied)
	}
	return err
}
