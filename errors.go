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
