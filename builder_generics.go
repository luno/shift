package shift

type optionCustom func(*options)

// NewGenericFSM returns a new FSM builder.
func NewGenericFSM[T Primary](events eventInserterT[T], opts ...optionCustom) initerT[T] {
	fsm := FSMT[T]{
		states: make(map[int]status),
		events: events,
	}

	for _, opt := range opts {
		opt(&fsm.options)
	}

	return initerT[T](fsm)
}

type builderT[T Primary] FSMT[T]

type initerT[T Primary] FSMT[T]

// Insert returns an FSM builder with the provided insert status.
func (c initerT[T]) Insert(st Status, inserter inserter[T], next ...Status) builderT[T] {
	c.states[st.ShiftStatus()] = status{
		st:     st,
		req:    inserter,
		t:      st,
		insert: false,
		next:   toMap(next),
	}
	c.insertStatus = st
	return builderT[T](c)
}

// Update returns an FSM builder with the provided status update added.
func (b builderT[T]) Update(st Status, updater updater[T], next ...Status) builderT[T] {
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
func (b builderT[T]) Build() *FSMT[T] {
	fsm := FSMT[T](b)
	return &fsm
}
