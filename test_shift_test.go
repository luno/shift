package shift_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
	"github.com/stretchr/testify/require"
)

//go:generate shiftgen -inserter=i -updaters=u -table=tests -out=gen_2_test.go

type i struct {
	I1 int64
	I2 string
	I3 time.Time
}

type u struct {
	ID int64
	U1 bool
	U2 Currency
	U3 sql.NullTime
	U4 sql.NullString
	U5 []byte
}

// TestTestFSM tests the TestFSM functionality which tests FSM instances
// by driving it through all state changes with fuzzed data.
func TestTestFSM(t *testing.T) {
	cases := []struct {
		name string
		fsm  *shift.FSM
		err  string
	}{
		{
			name: "insert only",
			fsm: shift.NewFSM(events).
				Insert(s(1), i{}).
				Build(),
		},
		{
			name: "insert update",
			fsm: shift.NewFSM(events).
				Insert(s(1), i{}, s(2)).
				Update(s(2), u{}).
				Build(),
		},
		{
			name: "update not reachable",
			fsm: shift.NewFSM(events).
				Insert(s(1), i{}).
				Update(s(2), u{}).
				Build(),
			err: "status not reachable",
		},
		{
			name: "cycle",
			fsm: shift.NewFSM(events).
				Insert(s(1), i{}, s(2)).
				Update(s(2), u{}, s(1)).
				Build(),
		},
		{
			name: "loop",
			fsm: shift.NewFSM(events).
				Insert(s(1), i{}, s(2)).
				Update(s(2), u{}, s(2)).
				Build(),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dbc := setup(t)

			err := shift.TestFSM(t, dbc, test.fsm)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.err)
			}
		})
	}
}

func (ii i) GetMetadata(ctx context.Context, tx *sql.Tx, id int64, status shift.Status) ([]byte, error) {
	return []byte(fmt.Sprint(id)), nil
}

func (uu u) GetMetadata(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) ([]byte, error) {
	return []byte(fmt.Sprint(uu.ID)), nil
}

func TestWithMeta(t *testing.T) {
	dbc := setup(t)
	defer dbc.Close()

	events = events.Clone(rsql.WithEventMetadataField("metadata"))

	fsm := shift.NewFSM(events, shift.WithMetadata()).
		Insert(s(1), i{}, s(2)).
		Update(s(2), u{}).
		Build()

	err := shift.TestFSM(t, dbc, fsm)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc, err := events.ToStream(dbc)(context.Background(), "")
	require.NoError(t, err)

	var c int
	err = dbc.QueryRowContext(ctx, "select count(*) from events").Scan(&c)
	require.NoError(t, err)
	require.Equal(t, 2, c)

	e, err := sc.Recv()
	require.NoError(t, err)
	require.True(t, reflex.IsType(s(1), e.Type))
	require.Equal(t, e.ForeignID, string(e.MetaData))

	e, err = sc.Recv()
	require.NoError(t, err)
	require.True(t, reflex.IsType(s(2), e.Type))
	require.Equal(t, e.ForeignID, string(e.MetaData))
}

func s(i int) shift.Status {
	return TestStatus(i)
}
