package shift_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
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
	U3 mysql.NullTime
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
			defer dbc.Close()

			err := shift.TestFSM(t, dbc, test.fsm)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.err)
			}
		})
	}
}

func s(i int) shift.Status {
	return TestStatus(i)
}
