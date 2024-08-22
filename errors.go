package shift

import (
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

// ErrRowCount is returned by generated shift code when an
// update failed due unexpected number of rows updated (n != 1).
// This is usually due to the row not being in the expected from
// state anymore.
var ErrRowCount = errors.New("unexpected number of rows updated", j.C("ERR_fcb8af57223847b1"))

// ErrUnknownStatus indicates that the status hasn't been registered
// with the FSM.
var ErrUnknownStatus = errors.New("unknown status", j.C("ERR_198a4c2d8a654b17"))

// ErrInvalidStateTransition indicates a state transition that hasn't been
// registered with the FSM.
var ErrInvalidStateTransition = errors.New("invalid state transition", j.C("ERR_be8211db784bfb67"))

// ErrInvalidType indicates that the provided request type isn't valid, and can't be
// used for the requested transition.
var ErrInvalidType = errors.New("invalid type", j.C("ERR_baf1a1f2e99951ec"))
