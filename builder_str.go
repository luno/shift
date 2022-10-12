package shift

type optionCustom func(*options)

// NewFSMWithStringPrimary returns a new FSM builder.
func NewFSMWithStringPrimary(events eventInserterStr, opts ...optionCustom) initerStr {
	fsm := FSMString{
		states: make(map[int]status),
		events: events,
	}

	for _, opt := range opts {
		opt(&fsm.options)
	}

	return initerStr(fsm)
}

type builderStr FSMString

type initerStr FSMString

// Insert returns an FSM builder with the provided insert status.
func (c initerStr) Insert(st Status, inserter inserterStr, next ...Status) builderStr {
	c.states[st.ShiftStatus()] = status{
		st:     st,
		req:    inserter,
		t:      st,
		insert: false,
		next:   toMap(next),
	}
	c.insertStatus = st
	return builderStr(c)
}

// Update returns an FSM builder with the provided status update added.
func (b builderStr) Update(st Status, updater updaterStr, next ...Status) builderStr {
	if _, has := b.states[st.ShiftStatus()]; has {
		// Ok to panic since it is build time.
		panic("state already added")
	}
	b.states[st.ShiftStatus()] = status{
		st:     st,
		req:    updater,
		t:      st,
		insert: false,
		next:   toMap(next),
	}
	return b
}

// Build returns the built FSM.
func (b builderStr) Build() *FSMString {
	fsm := FSMString(b)
	return &fsm
}
