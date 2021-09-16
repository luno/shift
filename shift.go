// Package shift provides the persistence layer for a simple "finite state machine"
// domain model with validation, explicit fields and reflex events per state change.
//
// shift.NewFSM builds a FSM instance that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event. Note that FSM is opinionated and has the following
// restrictions: only a single insert status, no transitions back to
// insert status, only a single transition per pair of statuses.
//
// shift.NewArcFSM builds a ArcFSM instance which is the same as an FSM
// but without its restrictions. It supports arbitrary transitions.
package shift

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/luno/jettison/errors"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

// Status is an individual state in the FSM.
//
// The canonical implementation is:
//	type MyStatus int
//	func (s MyStatus) ShiftStatus() int {
//		return int(s)
//	}
//	func (s MyStatus) ReflexType() int {
//		return int(s)
//	}
//	const (
//		StatusUnknown MyStatus = 0
//		StatusInsert  MyStatus = 1
//	)
type Status interface {
	ShiftStatus() int
	ReflexType() int
}

// Inserter provides an interface for inserting new state machine instance rows.
type Inserter interface {
	// Insert inserts a new row with status and returns an id or an error.
	Insert(ctx context.Context, tx *sql.Tx, status Status) (int64, error)
}

// Updater provides an interface for updating existing state machine instance rows.
type Updater interface {
	// Update updates the status of an existing row returns an id or an error.
	Update(ctx context.Context, tx *sql.Tx, from Status, to Status) (int64, error)
}

// MetadataInserter extends Inserter with additional metadata inserted with the reflex event.
type MetadataInserter interface {
	Inserter

	// GetMetadata returns the metadata to be inserted with the reflex event for the insert.
	GetMetadata(ctx context.Context, tx *sql.Tx, id int64, status Status) ([]byte, error)
}

// MetadataUpdater extends Updater with additional metadata inserted with the reflex event.
type MetadataUpdater interface {
	Updater

	// GetMetadata returns the metadata to be inserted with the reflex event for the update.
	GetMetadata(ctx context.Context, tx *sql.Tx, from Status, to Status) ([]byte, error)
}

// ValidatingInserter extends Inserter with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type ValidatingInserter interface {
	Inserter

	// Validate returns an error if the insert is not valid.
	Validate(ctx context.Context, tx *sql.Tx, id int64, status Status) error
}

// ValidatingUpdater extends Updater with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type ValidatingUpdater interface {
	Updater

	// Validate returns an error if the update is not valid.
	Validate(ctx context.Context, tx *sql.Tx, from Status, to Status) error
}

// eventInserter inserts reflex events into a sql DB table.
// It is implemented by rsql.EventsTableInt.
type eventInserter interface {
	InsertWithMetadata(ctx context.Context, tx *sql.Tx, foreignID int64,
		typ reflex.EventType, metadata []byte) (rsql.NotifyFunc, error)
}

// FSM is a defined Finite-State-Machine that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
//
// Note that this FSM is opinionated and has the following
// restrictions: only a single insert status, no transitions back to
// insert status, only a single transition per pair of statuses.
type FSM struct {
	options
	events       eventInserter
	states       map[int]status
	insertStatus Status
}

// Insert returns the id of the newly inserted domain model.
func (fsm *FSM) Insert(ctx context.Context, dbc *sql.DB, inserter Inserter) (int64, error) {
	tx, err := dbc.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	id, notify, err := fsm.InsertTx(ctx, tx, inserter)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	notify()
	return id, nil
}

func (fsm *FSM) InsertTx(ctx context.Context, tx *sql.Tx, inserter Inserter) (int64, rsql.NotifyFunc, error) {
	var (
		st = fsm.insertStatus
	)
	if !sameType(fsm.states[st.ShiftStatus()].req, inserter) {
		return 0, nil, errors.Wrap(ErrInvalidType, "inserter can't be used for this transition")
	}

	return insertTx(ctx, tx, st, inserter, fsm.events, fsm.states[st.ShiftStatus()].t, fsm.options)
}

func (fsm *FSM) Update(ctx context.Context, dbc *sql.DB, from Status, to Status, updater Updater) error {
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

func (fsm *FSM) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater) (rsql.NotifyFunc, error) {
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

	return updateTx(ctx, tx, from, to, updater, fsm.events, t.t, fsm.options)
}

func insertTx(ctx context.Context, tx *sql.Tx, st Status, inserter Inserter,
	events eventInserter, eventType reflex.EventType, opts options) (int64, rsql.NotifyFunc, error) {

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return 0, nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := inserter.(MetadataInserter)
		if !ok {
			return 0, nil, errors.Wrap(ErrInvalidType, "inserter without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, id, st)
		if err != nil {
			return 0, nil, err
		}
	}

	notify, err := events.InsertWithMetadata(ctx, tx, id, eventType, metadata)
	if err != nil {
		return 0, nil, err
	}

	if opts.withValidation {
		validate, ok := inserter.(ValidatingInserter)
		if !ok {
			return 0, nil, errors.Wrap(ErrInvalidType, "inserter without validate method")
		}

		err = validate.Validate(ctx, tx, id, st)
		if err != nil {
			return 0, nil, err
		}
	}

	return id, notify, err
}

func updateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater,
	events eventInserter, eventType reflex.EventType, opts options) (rsql.NotifyFunc, error) {

	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := updater.(MetadataUpdater)
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
		validate, ok := updater.(ValidatingUpdater)
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

type status struct {
	st     Status
	t      reflex.EventType
	req    interface{}
	insert bool
	next   map[Status]bool
}

func sameType(a interface{}, b interface{}) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}
