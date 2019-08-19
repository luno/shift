package shift

import (
	"github.com/luno/reflex/rsql"
)

// TODO(corver): Possibly support explicit shifting to status X from different
//  statuses (Y and Z) each with different requests (XFromYReq, XFromZReq).

// NewFSM returns a new FSM builder.
func NewFSM(events rsql.EventsTableInt) initer {
	return initer(builder{
		states: make(map[Status]status),
		events: events,
	})
}

type builder FSM

type initer builder

// Insert returns a FSM builder with the provided insert status.
func (c initer) Insert(st Status, stInserter Inserter, nextAllowed ...Status) builder {
	c.states[st] = status{
		st:     st,
		req:    stInserter,
		t:      st,
		insert: false,
		next:   toMap(nextAllowed),
	}
	c.insertStatus = st
	return builder(c)
}

// Update returns a FSM builder with the provided status update added.
func (b builder) Update(st Status, stUpdater Updater, nextAllowed ...Status) builder {
	if _, has := b.states[st]; has {
		// Ok to panic since it is build time.
		panic("state already added")
	}
	b.states[st] = status{
		st:     st,
		req:    stUpdater,
		t:      st,
		insert: false,
		next:   toMap(nextAllowed),
	}
	return b
}

// Build returns the built FSM.
func (b builder) Build() *FSM {
	fsm := FSM(b)
	return &fsm
}

func toMap(sl []Status) map[Status]bool {
	m := make(map[Status]bool)
	for _, s := range sl {
		m[s] = true
	}
	return m
}
