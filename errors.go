package shift

import (
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

var (
	ErrRowCount          = errors.New("unexpected number of rows updated", j.C("ERR_fcb8af57223847b1"))
	errUnknownStatus     = errors.New("unknown status", j.C("ERR_6adf3f41079e9c2a"))
	errMismatchStatusReq = errors.New("mismatching status and req", j.C("ERR_6ff328eaeb8727c9"))
	errInvalidTransition = errors.New("invalid transition", j.C("ERR_c62aebca2c6fe704"))
)
