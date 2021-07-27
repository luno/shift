package shift

type option func(*options)

type options struct {
	withMetadata   bool
	withValidation bool
}

// WithMetadata provides an option to enable event metadata with a FSM.
func WithMetadata() option {
	return func(o *options) {
		o.withMetadata = true
	}
}

// WithValidation provides an option to enable insert/update validation.
func WithValidation() option {
	return func(o *options) {
		o.withValidation = true
	}
}

// NewFSM returns a new FSM builder.
func NewFSM(events eventInserter, opts ...option) initer {
	fsm := FSM{
		states: make(map[int]status),
		events: events,
	}

	for _, opt := range opts {
		opt(&fsm.options)
	}

	return initer(builder(fsm))
}

type builder FSM

type initer builder

// Insert returns a FSM builder with the provided insert status.
func (c initer) Insert(st Status, inserter Inserter, next ...Status) builder {
	c.states[st.ShiftStatus()] = status{
		st:     st,
		req:    inserter,
		t:      st,
		insert: false,
		next:   toMap(next),
	}
	c.insertStatus = st
	return builder(c)
}

// Update returns a FSM builder with the provided status update added.
func (b builder) Update(st Status, updater Updater, next ...Status) builder {
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
