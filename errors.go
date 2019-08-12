package shift

import (
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

var (
	errUnknownStatus     = errors.New("unknown status", j.C("ERR_6adf3f41079e9c2a"))
	errMismatchStatusReq = errors.New("mismatching status and req", j.C("ERR_6ff328eaeb8727c9"))
	errInvalidTransition = errors.New("invalid transition", j.C("ERR_c62aebca2c6fe704"))
)
