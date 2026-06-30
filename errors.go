package sdk

import "errors"

// Silo error constants as defined in SILO_BRAIN.md §15.3.
var (
	ErrPathNotFound      = errors.New("path_not_found")
	ErrInvalidAPIKey     = errors.New("invalid_api_key")
	ErrWorkspaceInactive = errors.New("workspace_inactive")
	ErrRateLimitExceeded = errors.New("rate_limit_exceeded")
	ErrChecksumInvalid   = errors.New("checksum_invalid")
	ErrVersionConflict   = errors.New("version_conflict")
)

// MapErrorCode maps HTTP error strings to SDK error variables.
func MapErrorCode(errStr string) error {
	switch errStr {
	case "path_not_found":
		return ErrPathNotFound
	case "invalid_api_key":
		return ErrInvalidAPIKey
	case "workspace_inactive":
		return ErrWorkspaceInactive
	case "rate_limit_exceeded":
		return ErrRateLimitExceeded
	case "checksum_invalid":
		return ErrChecksumInvalid
	case "version_conflict":
		return ErrVersionConflict
	default:
		return errors.New(errStr)
	}
}
