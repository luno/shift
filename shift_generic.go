package shift

import (
	"context"
	"database/sql"
	"github.com/luno/jettison/errors"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

type Primary interface {
	int64 | string
}

// inserter provides an interface for inserting new state machine instance rows.
type inserter[T Primary] interface {
	// Insert inserts a new row with status and returns an id or an error.
	Insert(ctx context.Context, tx *sql.Tx, status Status) (T, error)
}

// updater provides an interface for updating existing state machine instance rows.
type updater[T Primary] interface {
	// Update updates the status of an existing row returns an id or an error.
	Update(ctx context.Context, tx *sql.Tx, from Status, to Status) (T, error)
}

// metadataInserter extends inserter with additional metadata inserted with the reflex event.
type metadataInserter[T Primary] interface {
	inserter[T]

	// GetMetadata returns the metadata to be inserted with the reflex event for the insert.
	GetMetadata(ctx context.Context, tx *sql.Tx, id T, status Status) ([]byte, error)
}

// metadataUpdater extends updater with additional metadata inserted with the reflex event.
type metadataUpdater[T Primary] interface {
	updater[T]

	// GetMetadata returns the metadata to be inserted with the reflex event for the update.
	GetMetadata(ctx context.Context, tx *sql.Tx, from Status, to Status) ([]byte, error)
}

// validatingInserter extends inserter with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type validatingInserter[T Primary] interface {
	inserter[T]

	// Validate returns an error if the insert is not valid.
	Validate(ctx context.Context, tx *sql.Tx, id T, status Status) error
}

// validatingUpdater extends updater with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type validatingUpdater[T Primary] interface {
	updater[T]

	// Validate returns an error if the update is not valid.
	Validate(ctx context.Context, tx *sql.Tx, from Status, to Status) error
}

// eventInserterT inserts reflex events into a sql DB table.
// It is implemented by rsql.EventsTable or rsql.EventsTableInt.
type eventInserterT[T Primary] interface {
	InsertWithMetadata(ctx context.Context, tx *sql.Tx, foreignID T,
		typ reflex.EventType, metadata []byte) (rsql.NotifyFunc, error)
}

// FSMT is a defined Finite-State-Machine that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
//
// Note that this FSM is opinionated and has the following
// restrictions: only a single insert status, no transitions back to
// insert status, only a single transition per pair of statuses.
type FSMT[T Primary] struct {
	options
	events       eventInserterT[T]
	states       map[int]status
	insertStatus Status
}

// Insert returns the id of the newly inserted domain model.
func (fsm *FSMT[T]) Insert(ctx context.Context, dbc *sql.DB, inserter inserter[T]) error {
	tx, err := dbc.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, notify, err := fsm.InsertTx(ctx, tx, inserter)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	notify()
	return nil
}

func (fsm *FSMT[T]) InsertTx(ctx context.Context, tx *sql.Tx, inserter inserter[T]) (T, rsql.NotifyFunc, error) {
	st := fsm.insertStatus
	if !sameType(fsm.states[st.ShiftStatus()].req, inserter) {
		var zeroT T
		return zeroT, nil, errors.Wrap(ErrInvalidType, "inserter can't be used for this transition")
	}

	return insertTxT[T](ctx, tx, st, inserter, fsm.events, fsm.states[st.ShiftStatus()].t, fsm.options)
}

func (fsm *FSMT[T]) Update(ctx context.Context, dbc *sql.DB, from Status, to Status, updater updater[T]) error {
	tx, err := dbc.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	notify, err := fsm.UpdateTx(ctx, tx, from, to, updater)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	notify()
	return nil
}

func (fsm *FSMT[T]) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater updater[T]) (rsql.NotifyFunc, error) {
	t, ok := fsm.states[to.ShiftStatus()]
	if !ok {
		return nil, errors.Wrap(ErrUnknownStatus, "unknown to status")
	}
	if !sameType(t.req, updater) {
		return nil, errors.Wrap(ErrInvalidType, "updater can't be used for this transition")
	}
	f, ok := fsm.states[from.ShiftStatus()]
	if !ok {
		return nil, errors.Wrap(ErrUnknownStatus, "unknown from status")
	} else if !f.next[to] {
		return nil, errors.Wrap(ErrInvalidStateTransition, "")
	}

	return updateTxT(ctx, tx, from, to, updater, fsm.events, t.t, fsm.options)
}

func insertTxT[T Primary](ctx context.Context, tx *sql.Tx, st Status, inserter inserter[T],
	events eventInserterT[T], eventType reflex.EventType, opts options) (T, rsql.NotifyFunc, error) {

	var zeroT T

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return zeroT, nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := inserter.(metadataInserter[T])
		if !ok {
			return zeroT, nil, errors.Wrap(ErrInvalidType, "inserter without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, id, st)
		if err != nil {
			return zeroT, nil, err
		}
	}

	notify, err := events.InsertWithMetadata(ctx, tx, id, eventType, metadata)
	if err != nil {
		return zeroT, nil, err
	}

	if opts.withValidation {
		validate, ok := inserter.(validatingInserter[T])
		if !ok {
			return zeroT, nil, errors.Wrap(ErrInvalidType, "inserter without validate method")
		}

		err = validate.Validate(ctx, tx, id, st)
		if err != nil {
			return zeroT, nil, err
		}
	}

	return id, notify, err
}

func updateTxT[T Primary](ctx context.Context, tx *sql.Tx, from Status, to Status, updater updater[T],
	events eventInserterT[T], eventType reflex.EventType, opts options) (rsql.NotifyFunc, error) {

	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := updater.(metadataUpdater[T])
		if !ok {
			return nil, errors.Wrap(ErrInvalidType, "updater without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, from, to)
		if err != nil {
			return nil, err
		}
	}

	notify, err := events.InsertWithMetadata(ctx, tx, id, eventType, metadata)
	if err != nil {
		return nil, err
	}

	if opts.withValidation {
		validate, ok := updater.(validatingUpdater[T])
		if !ok {
			return nil, errors.Wrap(ErrInvalidType, "updater without validate method")
		}

		err = validate.Validate(ctx, tx, from, to)
		if err != nil {
			return nil, err
		}
	}

	return notify, nil
}
