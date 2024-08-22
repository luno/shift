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
	"fmt"
	"reflect"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

// Status is an individual state in the FSM.
//
// The canonical implementation is:
//
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

type primary interface {
	int64 | string
}

// Inserter provides an interface for inserting new state machine instance rows.
type Inserter[T primary] interface {
	// Insert inserts a new row with status and returns an id or an error.
	Insert(ctx context.Context, tx *sql.Tx, status Status) (T, error)
}

// Updater provides an interface for updating existing state machine instance rows.
type Updater[T primary] interface {
	// Update updates the status of an existing row returns an id or an error.
	Update(ctx context.Context, tx *sql.Tx, from Status, to Status) (T, error)
}

// MetadataInserter extends inserter with additional metadata inserted with the reflex event.
type MetadataInserter[T primary] interface {
	Inserter[T]

	// GetMetadata returns the metadata to be inserted with the reflex event for the insert.
	GetMetadata(ctx context.Context, tx *sql.Tx, id T, status Status) ([]byte, error)
}

// MetadataUpdater extends updater with additional metadata inserted with the reflex event.
type MetadataUpdater[T primary] interface {
	Updater[T]

	// GetMetadata returns the metadata to be inserted with the reflex event for the update.
	GetMetadata(ctx context.Context, tx *sql.Tx, from Status, to Status) ([]byte, error)
}

// ValidatingInserter extends inserter with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type ValidatingInserter[T primary] interface {
	Inserter[T]

	// Validate returns an error if the insert is not valid.
	Validate(ctx context.Context, tx *sql.Tx, id T, status Status) error
}

// ValidatingUpdater extends updater with validation. Assuming the majority
// validations will be successful, the validation is done after event insertion
// to allow maximum flexibility sacrificing invalid path performance.
type ValidatingUpdater[T primary] interface {
	Updater[T]

	// Validate returns an error if the update is not valid.
	Validate(ctx context.Context, tx *sql.Tx, from Status, to Status) error
}

// eventInserter inserts reflex events into a sql DB table.
// It is implemented by rsql.EventsTable or rsql.EventsTableInt.
type eventInserter[T primary] interface {
	InsertWithMetadata(ctx context.Context, tx *sql.Tx, foreignID T,
		typ reflex.EventType, metadata []byte) (rsql.NotifyFunc, error)
}

type FSM = GenFSM[int64]

// GenFSM is a defined Finite-State-Machine that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
//
// The type of the GenFSM is the type of the primary key used by the user table.
//
// Note that this FSM is opinionated and has the following
// restrictions: only a single insert status, no transitions back to
// insert status, only a single transition per pair of statuses.
type GenFSM[T primary] struct {
	options
	events       eventInserter[T]
	states       map[int]status
	insertStatus Status
}

// Insert returns the id of the newly inserted domain model.
func (fsm *GenFSM[T]) Insert(ctx context.Context, dbc *sql.DB, inserter Inserter[T]) (T, error) {
	var zeroT T
	tx, err := dbc.Begin()
	if err != nil {
		return zeroT, err
	}
	defer tx.Rollback()

	id, notify, err := fsm.InsertTx(ctx, tx, inserter)
	if err != nil {
		return zeroT, err
	}

	err = tx.Commit()
	if err != nil {
		return zeroT, err
	}

	notify()
	return id, nil
}

func (fsm *GenFSM[T]) InsertTx(ctx context.Context, tx *sql.Tx, inserter Inserter[T]) (T, rsql.NotifyFunc, error) {
	st := fsm.insertStatus
	if !sameType(fsm.states[st.ShiftStatus()].req, inserter) {
		var zeroT T
		return zeroT, nil, errors.Wrap(ErrInvalidType, "inserter can't be used for this transition")
	}

	return insertTx[T](ctx, tx, st, inserter, fsm.events, fsm.states[st.ShiftStatus()].t, fsm.options)
}

func (fsm *GenFSM[T]) Update(ctx context.Context, dbc *sql.DB, from Status, to Status, updater Updater[T]) error {
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

func (fsm *GenFSM[T]) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater[T]) (rsql.NotifyFunc, error) {
	t, ok := fsm.states[to.ShiftStatus()]
	if !ok {
		return nil, errors.Wrap(ErrUnknownStatus, "unknown 'to' status", j.MKV{"from": fmt.Sprintf("%v", from), "to": fmt.Sprintf("%v", to)})
	}
	if !sameType(t.req, updater) {
		return nil, errors.Wrap(ErrInvalidType, "updater can't be used for this transition")
	}
	f, ok := fsm.states[from.ShiftStatus()]
	if !ok {
		return nil, errors.Wrap(ErrUnknownStatus, "unknown 'from' status", j.MKV{"from": fmt.Sprintf("%v", from), "to": fmt.Sprintf("%v", to)})
	} else if !f.next[to] {
		return nil, errors.Wrap(ErrInvalidStateTransition, "", j.MKV{"from": fmt.Sprintf("%v", from), "to": fmt.Sprintf("%v", to)})
	}

	return updateTx(ctx, tx, from, to, updater, fsm.events, t.t, fsm.options)
}

func insertTx[T primary](ctx context.Context, tx *sql.Tx, st Status, inserter Inserter[T],
	events eventInserter[T], eventType reflex.EventType, opts options,
) (T, rsql.NotifyFunc, error) {
	var zeroT T

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return zeroT, nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := inserter.(MetadataInserter[T])
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
		validate, ok := inserter.(ValidatingInserter[T])
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

func updateTx[T primary](ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater[T],
	events eventInserter[T], eventType reflex.EventType, opts options,
) (rsql.NotifyFunc, error) {
	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	var metadata []byte
	if opts.withMetadata {
		meta, ok := updater.(MetadataUpdater[T])
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
		validate, ok := updater.(ValidatingUpdater[T])
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
