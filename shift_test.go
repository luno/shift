package shift_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/stretchr/testify/require"

	"github.com/luno/shift"
)

//go:generate go run github.com/luno/shift/shiftgen -inserter=insert -updaters=update,complete -table=users -out=gen_1_test.go

type insert struct {
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
}

type update struct {
	ID     int64
	Name   string
	Amount Currency
}

type complete struct {
	ID int64
}

type TestStatus int

func (s TestStatus) ShiftStatus() int {
	return int(s)
}

func (s TestStatus) ReflexType() int {
	return int(s)
}

const (
	StatusInit     TestStatus = 1
	StatusUpdate   TestStatus = 2
	StatusComplete TestStatus = 3
)

const usersTable = "users"

var (
	events = rsql.NewEventsTableInt("events", rsql.WithoutEventsCache())
	fsm    = shift.NewFSM(events).
		Insert(StatusInit, insert{}, StatusUpdate).
		Update(StatusUpdate, update{}, StatusComplete).
		Update(StatusComplete, complete{}).
		Build()
)

func TestAboveFSM(t *testing.T) {
	dbc := setup(t)

	jtest.RequireNil(t, shift.TestFSM(t, dbc, fsm))
}

func TestBasic(t *testing.T) {
	dbc := setup(t)

	t0 := time.Now().Truncate(time.Second)
	amount := Currency{Valid: true, Amount: 99}
	ctx := context.Background()

	// Init model
	id, err := fsm.Insert(ctx, dbc, insert{Name: "insertMe", DateOfBirth: t0})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id, "insertMe", t0, Currency{}, 1)

	// Update model
	err = fsm.Update(ctx, dbc, StatusInit, StatusUpdate, update{ID: id, Name: "updateMe", Amount: amount})
	jtest.RequireNil(t, err)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id, "updateMe", t0, amount, 1, 2)

	// Complete model
	err = fsm.Update(ctx, dbc, StatusUpdate, StatusComplete, complete{ID: id})
	jtest.RequireNil(t, err)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id, "updateMe", t0, amount, 1, 2, 3)
}

func assertUser(t *testing.T, dbc *sql.DB, stream reflex.StreamFunc, table string,
	id any, exName string, exDOB time.Time, exAmount Currency, exEvents ...TestStatus,
) {
	var name sql.NullString
	var amount Currency
	var dob time.Time
	err := dbc.QueryRow("select name, dob, amount "+
		"from "+table+" where id=?", id).Scan(&name, &dob, &amount)
	jtest.RequireNil(t, err)
	require.Equal(t, exName, name.String)
	require.Equal(t, exDOB.UTC(), dob.UTC())
	require.Equal(t, exAmount, amount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sc, err := stream(ctx, "")
	jtest.RequireNil(t, err)
	for _, exE := range exEvents {
		e, err := sc.Recv()
		jtest.RequireNil(t, err)
		require.Equal(t, int(exE), e.Type.ReflexType())
	}
}

//go:generate go run github.com/luno/shift/shiftgen -inserter=insertStr -updaters=updateStr,completeStr -table=usersStr -out=gen_string_test.go

type insertStr struct {
	ID          string
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
}

type updateStr struct {
	ID     string
	Name   string
	Amount Currency
}

type completeStr struct {
	ID string
}

const usersStrTable = "usersStr"

var (
	eventsStr = rsql.NewEventsTable("eventsStr")
	fsmStr    = shift.NewGenFSM[string](eventsStr).
			Insert(StatusInit, insertStr{}, StatusUpdate).
			Update(StatusUpdate, updateStr{}, StatusComplete).
			Update(StatusComplete, completeStr{}).
			Build()
)

func TestBasic_StringFSM(t *testing.T) {
	dbc := setup(t)

	t0 := time.Now().Truncate(time.Second)
	amount := Currency{Valid: true, Amount: 99}
	ctx := context.Background()

	// Init model
	id, err := fsmStr.Insert(ctx, dbc, insertStr{ID: "abcdef123456", Name: "insertMe", DateOfBirth: t0})
	jtest.RequireNil(t, err)
	require.Equal(t, "abcdef123456", id)

	assertUser(t, dbc, eventsStr.ToStream(dbc), usersStrTable, id, "insertMe", t0, Currency{}, 1)

	// Update model
	err = fsmStr.Update(ctx, dbc, StatusInit, StatusUpdate, updateStr{ID: id, Name: "updateMe", Amount: amount})
	jtest.RequireNil(t, err)

	assertUser(t, dbc, eventsStr.ToStream(dbc), usersStrTable, id, "updateMe", t0, amount, 1, 2)

	// Complete model
	err = fsmStr.Update(ctx, dbc, StatusUpdate, StatusComplete, completeStr{ID: id})
	jtest.RequireNil(t, err)

	assertUser(t, dbc, eventsStr.ToStream(dbc), usersStrTable, id, "updateMe", t0, amount, 1, 2, 3)
}

func (ii i) Validate(ctx context.Context, tx *sql.Tx, id int64, status shift.Status) error {
	if id > 1 {
		return errInsertInvalid
	}
	return nil
}

func (uu u) Validate(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) error {
	if from.ShiftStatus() == to.ShiftStatus() {
		return errUpdateInvalid
	}
	return nil
}

var (
	errInsertInvalid = errors.New("only single row permitted", j.C("ERR_d9ec7823de79aa28"))
	errUpdateInvalid = errors.New("only single row permitted", j.C("ERR_e67f85dcb425e083"))
)

func TestWithValidation(t *testing.T) {
	dbc := setup(t)
	defer dbc.Close()

	fsm := shift.NewFSM(events, shift.WithValidation()).
		Insert(s(1), i{}, s(2)).
		Update(s(2), u{}, s(2)). // Allow 2 -> 2 update, validation will fail.
		Build()

	ctx := context.Background()

	// First insert is ok
	id, err := fsm.Insert(ctx, dbc, i{I3: time.Now()})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id)

	// Second insert fails.
	_, err = fsm.Insert(ctx, dbc, i{I3: time.Now()})
	jtest.Require(t, errInsertInvalid, err)

	// Update from 1 -> 2 is ok
	err = fsm.Update(ctx, dbc, s(1), s(2), u{ID: id})
	jtest.RequireNil(t, err)

	// Update from 2 -> 2 fails
	err = fsm.Update(ctx, dbc, s(2), s(2), u{ID: id, U1: true})
	jtest.Require(t, errUpdateInvalid, err)
}

//go:generate go run github.com/luno/shift/shiftgen -inserter=i_t -updaters=u_t -table=tests -out=gen_3_test.go

type i_t struct {
	I1        int64
	I2        string
	I3        time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type u_t struct {
	ID        int64
	U1        bool
	U2        Currency
	U3        sql.NullTime
	U4        sql.NullString
	U5        []byte
	UpdatedAt time.Time
}

func TestWithTimestamps(t *testing.T) {
	dbc := setup(t)
	defer dbc.Close()

	fsm := shift.NewFSM(events).
		Insert(s(1), i_t{}, s(2)).
		Update(s(2), u_t{}, s(2)). // Allow 2 -> 2 update, validation will fail.
		Build()

	ctx := context.Background()
	t0 := time.Now()

	id, err := fsm.Insert(ctx, dbc, i_t{I3: time.Now(), UpdatedAt: t0})
	require.Error(t, err, "created_at is required")
	require.Zero(t, 0)

	id, err = fsm.Insert(ctx, dbc, i_t{I3: time.Now(), CreatedAt: t0})
	require.Error(t, err, "updated_at is required")
	require.Zero(t, 0)

	// First insert is ok
	id, err = fsm.Insert(ctx, dbc, i_t{I3: time.Now(), CreatedAt: t0, UpdatedAt: t0})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id)

	err = fsm.Update(ctx, dbc, s(1), s(2), u_t{ID: id})
	require.Error(t, err, "updated_at is required")

	// Update from 1 -> 2 is ok
	err = fsm.Update(ctx, dbc, s(1), s(2), u_t{ID: id, UpdatedAt: t0})
	jtest.RequireNil(t, err)
}

func TestGenFSM_Update(t *testing.T) {
	dbc := setup(t)

	t0 := time.Now().Truncate(time.Second)
	amount := Currency{Valid: true, Amount: 99}
	ctx := context.Background()

	// Init model
	id, err := fsm.Insert(ctx, dbc, insert{Name: "insertMe", DateOfBirth: t0})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id, "insertMe", t0, Currency{}, 1)

	var unknownShiftStatus TestStatus = 999
	tests := []struct {
		name        string
		from        shift.Status
		to          shift.Status
		expectedErr error
	}{
		{
			name: "Valid",
			from: StatusInit,
			to:   StatusUpdate,
		},
		{
			name:        "Invalid State Transition",
			from:        StatusComplete,
			to:          StatusUpdate,
			expectedErr: shift.ErrInvalidStateTransition,
		},
		{
			name:        "Unknown 'from' status",
			from:        unknownShiftStatus,
			to:          StatusUpdate,
			expectedErr: errors.Wrap(shift.ErrUnknownStatus, "unknown 'from' status", j.MKV{"from ": fmt.Sprintf("%T", unknownShiftStatus), "to": fmt.Sprintf("%T", StatusUpdate)}),
		},
		{
			name:        "Unknown 'to' status",
			from:        StatusUpdate,
			to:          unknownShiftStatus,
			expectedErr: errors.Wrap(shift.ErrUnknownStatus, "unknown 'to' status", j.MKV{"from ": fmt.Sprintf("%T", StatusUpdate), "to": fmt.Sprintf("%T", unknownShiftStatus)}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fsm.Update(ctx, dbc, tt.from, tt.to, update{ID: id, Name: "updateMe", Amount: amount})
			jtest.Assert(t, tt.expectedErr, err)
		})
	}
}
