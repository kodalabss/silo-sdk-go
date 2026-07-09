package silo

import (
	"errors"
	"fmt"
)

var (
	// Identity errors
	ErrIdAuthFail        = errors.New("ID_AUTH_FAIL: Invalid credentials or token")
	ErrIdProvisionExists = errors.New("ID_PROVISION_EXISTS: Workspace name already reserved")
	ErrIdSessionExpired  = errors.New("ID_SESSION_EXPIRED: Session timed out")

	// Geometry errors
	ErrGePathInvalid    = errors.New("GE_PATH_INVALID: Invalid path format")
	ErrGeCoordinateVoid = errors.New("GE_COORDINATE_VOID: Path not found")
	ErrGeSignatureDrift = errors.New("GE_SIGNATURE_DRIFT: State out of sync, call Sync")

	// Substance errors
	ErrSuVersionConflict = errors.New("SU_VERSION_CONFLICT: Expected generation mismatch")
	ErrSuCorruption      = errors.New("SU_CORRUPTION: Data integrity verification failed")
	ErrSuCapacityFull    = errors.New("SU_CAPACITY_FULL: Storage quota exceeded")

	// Navigation errors
	ErrNaWfpEmpty    = errors.New("NA_WFP_EMPTY: No results found")
	ErrNaRdmMismatch = errors.New("NA_RDM_MISMATCH: Invalid mapping range")

	// Traffic errors
	ErrTrBlackout  = errors.New("TR_BLACKOUT: Target workspace is unavailable")
	ErrTrRateLimit = errors.New("TR_RATE_LIMIT: Rate limit exceeded")

	// System errors
	ErrSyServiceDown = errors.New("SY_SERVICE_DOWN: Gateway unreachable")
	ErrSySpineBreak  = errors.New("SY_SPINE_BREAK: Internal storage error")
)

// MapErrorCode translates internal codes into standard Go errors.
func MapErrorCode(code string, coordinate uint64) error {
	var base error
	switch code {
	case "ID_AUTH_FAIL":
		base = ErrIdAuthFail
	case "ID_PROVISION_EXISTS":
		base = ErrIdProvisionExists
	case "ID_SESSION_EXPIRED":
		base = ErrIdSessionExpired
	case "GE_PATH_INVALID":
		base = ErrGePathInvalid
	case "GE_COORDINATE_VOID", "path_not_found":
		base = ErrGeCoordinateVoid
	case "GE_SIGNATURE_DRIFT":
		base = ErrGeSignatureDrift
	case "SU_VERSION_CONFLICT", "version_conflict":
		base = ErrSuVersionConflict
	case "SU_CORRUPTION":
		base = ErrSuCorruption
	case "SU_CAPACITY_FULL":
		base = ErrSuCapacityFull
	case "NA_WFP_EMPTY":
		base = ErrNaWfpEmpty
	case "NA_RDM_MISMATCH":
		base = ErrNaRdmMismatch
	case "TR_BLACKOUT":
		base = ErrTrBlackout
	case "TR_RATE_LIMIT", "rate_limit_exceeded":
		base = ErrTrRateLimit
	case "SY_SERVICE_DOWN":
		base = ErrSyServiceDown
	case "SY_SPINE_BREAK":
		base = ErrSySpineBreak
	default:
		return fmt.Errorf("ERROR_UNKNOWN: [%s] at Coordinate: %d", code, coordinate)
	}

	if coordinate != 0 {
		return fmt.Errorf("%w (Coord: %d)", base, coordinate)
	}
	return base
}
