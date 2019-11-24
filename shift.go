// Package shift provides the persistence layer for a simple "finite state machine"
// domain model with validation, explicit fields and reflex events per state change.
//
// shift.NewFSM builds a FSM instance that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
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
//	func (s MyStatus) ShiftStatus() {}
//	func (s MyStatus) Enum() int {
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
	Enum() int
	ShiftStatus()
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

// eventInserter inserts reflex events into a sql DB table.
// It is implemented by rsql.EventsTableInt.
type eventInserter interface {
	InsertWithMetadata(ctx context.Context, tx *sql.Tx, foreignID int64,
		typ reflex.EventType, metadata []byte) (rsql.NotifyFunc, error)
}

type FSM struct {
	events       eventInserter
	states       map[Status]status
	insertStatus Status
	withMetadata bool
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
	defer notify()

	return id, tx.Commit()
}

func (fsm *FSM) InsertTx(ctx context.Context, tx *sql.Tx, inserter Inserter) (int64, rsql.NotifyFunc, error) {
	var (
		st = fsm.insertStatus
	)
	if !sameType(fsm.states[st].req, inserter) {
		return 0, nil, errMismatchStatusReq
	}

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return 0, nil, err
	}

	var metadata []byte
	if fsm.withMetadata {
		meta, ok := inserter.(MetadataInserter)
		if !ok {
			return 0, nil, errors.New("inserter without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, id, st)
		if err != nil {
			return 0, nil, err
		}
	}

	notify, err := fsm.events.InsertWithMetadata(ctx, tx, id, fsm.states[st].t, metadata)
	if err != nil {
		return 0, nil, err
	}

	return id, notify, err
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
	defer notify()

	return tx.Commit()
}

func (fsm *FSM) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater) (rsql.NotifyFunc, error) {
	t, ok := fsm.states[to]
	if !ok {
		return nil, errUnknownStatus
	}
	if !sameType(t.req, updater) {
		return nil, errMismatchStatusReq
	}
	f, ok := fsm.states[from]
	if !ok {
		return nil, errUnknownStatus
	} else if !f.next[to] {
		return nil, errInvalidTransition
	}

	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	var metadata []byte
	if fsm.withMetadata {
		meta, ok := updater.(MetadataUpdater)
		if !ok {
			return nil, errors.New("updater without metadata")
		}

		var err error
		metadata, err = meta.GetMetadata(ctx, tx, from, to)
		if err != nil {
			return nil, err
		}
	}

	return fsm.events.InsertWithMetadata(ctx, tx, id, fsm.states[to].t, metadata)
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
