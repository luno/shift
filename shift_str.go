package shift

import (
	"context"
	"database/sql"
	"github.com/luno/jettison/errors"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

type inserterStr interface {
	// Insert inserts a new row with status and returns an id or an error.
	Insert(ctx context.Context, tx *sql.Tx, status Status) (string, error)
}

type updaterStr interface {
	// Update updates the status of an existing row returns an id or an error.
	Update(ctx context.Context, tx *sql.Tx, from Status, to Status) (string, error)
}

type metadataInserterStr interface {
	inserterStr

	// GetMetadata returns the metadata to be inserted with the reflex event for the insert.
	GetMetadata(ctx context.Context, tx *sql.Tx, id string, status Status) ([]byte, error)
}

type metadataUpdaterStr interface {
	updaterStr

	// GetMetadata returns the metadata to be inserted with the reflex event for the update.
	GetMetadata(ctx context.Context, tx *sql.Tx, from Status, to Status) ([]byte, error)
}

type validatingInserterStr interface {
	inserterStr

	// Validate returns an error if the insert is not valid.
	Validate(ctx context.Context, tx *sql.Tx, id string, status Status) error
}

type validatingUpdaterStr interface {
	updaterStr

	// Validate returns an error if the update is not valid.
	Validate(ctx context.Context, tx *sql.Tx, from Status, to Status) error
}

// eventInserter inserts reflex events into a sql DB table.
// It is implemented by rsql.EventsTable.
type eventInserterStr interface {
	InsertWithMetadata(ctx context.Context, tx *sql.Tx, foreignID string,
		typ reflex.EventType, metadata []byte) (rsql.NotifyFunc, error)
}

type FSMString struct {
	options
	events       eventInserterStr
	states       map[int]status
	insertStatus Status
}

func (fsm *FSMString) Insert(ctx context.Context, dbc *sql.DB, inserter inserterStr) error {
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

func (fsm *FSMString) InsertTx(ctx context.Context, tx *sql.Tx, inserter inserterStr) (string, rsql.NotifyFunc, error) {
	st := fsm.insertStatus
	if !sameType(fsm.states[st.ShiftStatus()].req, inserter) {
		return "", nil, errors.Wrap(ErrInvalidType, "inserter can't be used for this transition")
	}

	return insertTxStr(ctx, tx, st, inserter, fsm.events, fsm.states[st.ShiftStatus()].t, fsm.options)
}

func (fsm *FSMString) Update(ctx context.Context, dbc *sql.DB, from Status, to Status, updater updaterStr) error {
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

func (fsm *FSMString) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater updaterStr) (rsql.NotifyFunc, error) {
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

	return updateTxStr(ctx, tx, from, to, updater, fsm.events, t.t, fsm.options)
}

func insertTxStr(ctx context.Context, tx *sql.Tx, st Status, inserter inserterStr,
	events eventInserterStr, eventType reflex.EventType, opts options) (string, rsql.NotifyFunc, error) {

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return "", nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := inserter.(metadataInserterStr)
		if !ok {
			return "", nil, errors.Wrap(ErrInvalidType, "inserter without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, id, st)
		if err != nil {
			return "", nil, err
		}
	}

	notify, err := events.InsertWithMetadata(ctx, tx, id, eventType, metadata)
	if err != nil {
		return "", nil, err
	}

	if opts.withValidation {
		validate, ok := inserter.(validatingInserterStr)
		if !ok {
			return "", nil, errors.Wrap(ErrInvalidType, "inserter without validate method")
		}

		err = validate.Validate(ctx, tx, id, st)
		if err != nil {
			return "", nil, err
		}
	}

	return id, notify, err
}

func updateTxStr(ctx context.Context, tx *sql.Tx, from Status, to Status, updater updaterStr,
	events eventInserterStr, eventType reflex.EventType, opts options) (rsql.NotifyFunc, error) {

	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := updater.(metadataUpdaterStr)
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
		validate, ok := updater.(validatingUpdaterStr)
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
