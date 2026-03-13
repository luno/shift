package shift

type option func(*options)

type options struct {
	withMetadata   bool
	withValidation bool
	tableName      string // entity table name, set by WithVersion
}

// WithMetadata provides an option to enable event metadata with an FSM.
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

// WithVersion enables version tracking for the entity table. The tableName
// must match the sql table managed by this FSM. When enabled, after each
// insert or update, the entity's version is read and embedded in the reflex
// event metadata as a JSON-encoded record_version header.
//
// The entity table must have a `version` column and the events table must
// have metadata enabled via rsql.WithEventMetadataField.
//
// Use shiftgen -version to generate insert/update methods that set and
// increment the version column.
func WithVersion(tableName string) option {
	return func(o *options) {
		o.tableName = tableName
	}
}

// NewFSM returns a new FSM initer that supports a user table with an int64
// primary key.
func NewFSM(events eventInserter[int64], opts ...option) initer[int64] {
	return NewGenFSM[int64](events, opts...)
}

// NewGenFSM returns a new FSM initer. The type T should match the type of the
// user table's primary key.
func NewGenFSM[T primary](events eventInserter[T], opts ...option) initer[T] {
	fsm := GenFSM[T]{
		states: make(map[int]status),
		events: events,
	}

	for _, opt := range opts {
		opt(&fsm.options)
	}

	return initer[T](fsm)
}

// initer supports adding an inserter to the FSM.
type initer[T primary] GenFSM[T]

// Insert returns an FSM builder with the provided insert status.
func (c initer[T]) Insert(st Status, inserter Inserter[T], next ...Status) builder[T] {
	c.states[st.ShiftStatus()] = status{
		st:     st,
		req:    inserter,
		t:      st,
		insert: false,
		next:   toMap(next),
	}
	c.insertStatus = st
	return builder[T](c)
}

// builder supports adding an updater to the FSM.
type builder[T primary] GenFSM[T]

// Update returns an FSM builder with the provided status update added.
func (b builder[T]) Update(st Status, updater Updater[T], next ...Status) builder[T] {
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
func (b builder[T]) Build() *GenFSM[T] {
	fsm := GenFSM[T](b)
	return &fsm
}

func toMap(sl []Status) map[Status]bool {
	m := make(map[Status]bool)
	for _, s := range sl {
		m[s] = true
	}
	return m
}
