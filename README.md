# Shift
![Go](https://github.com/luno/shift/workflows/Go/badge.svg?branch=master) 
[![Go Report Card](https://goreportcard.com/badge/github.com/luno/shift?style=flat-square)](https://goreportcard.com/report/github.com/luno/shift)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/luno/shift)

Shift provides the SQL persistence layer for a simple "finite state machine" domain model. It provides validation, explicit fields and reflex events per state change. It is therefore used to explicitly define the life cycle of the domain model, i.e., the states it can transition through and the data modifications required for each transition.

# Overview

A Shift state machine is composed of an initial state followed by multiple subsequent states linked by allowed transitions, i.e., a rooted directed graph.

```mermaid
stateDiagram-v2
    direction LR
    [*] --> Created
    Created --> Pending
    Pending --> Failed
    Pending --> Completed
    Failed --> Pending
    Completed --> [*]
```
Each state has an associated struct defining the data modified when entering the state.

```go
type create struct {
  UserID string
  Type   int
}

type pending struct {
  ID int64
}

type failed struct {
  ID    int64
  Error string
}

type completed struct {
  ID     int64
  Result string
}
```

Some properties:                    
- States are instances of an enum implementing `shift.Status` interface.
- A state has an allowed set of next states.
- Only one state can be the initial state.
- All subsequent states are reached by explicit transitions from a state.
- Cycles are allowed; transitioning to an upstream state or even to itself.
- It is not allowed to transition to the initial state.
- Entering the initial state always inserts a new row.
- The initial state's struct may therefore not contain an ID field. 
- Entering a subsequent states always updates an existing row.
- Subsequent states' structs must therefore contain an ID field. 
- `int64` and `string` ID fields are supported.
- Created and updated times are guaranteed to be reliable:
  - By default, `time.Now()` is used to set the timestamp columns.
  - If specified in the inserter or updater, shift will use the provided time. This can be useful for testing.
  - Shift will error if a zero time is provided (i.e. if time is not set)
  - Columns must be named `created_at` and `updated_at`
- All transitions are recorded as [reflex](https://github.com/luno/reflex) events.

Differences of ArcFSM from FSM:
- For improved flexibility, ArcFSM was added without the transition restrictions of FSM.
- It supports arbitrary initial states and arbitrary transitions.

# Usage

The above state machine is defined by:
```go
events := rsql.NewEventsTableInt("events")
fsm := shift.NewFSM(events)
  Insert(CREATED, create{}, PENDING).
  Update(PENDING, pending{}, COMPLETED, FAILED).
  Update(FAILED, failed{}, PENDING).
  Update(COMPLETED, completed{}).
  Build()
  
// Note the format: STATE, struct{}, NEXT_STATE_A, NEXT_STATE_B    
```

Shift requires the state structs to implement `Inserter` or `Updater` interfaces which performs the actual SQL queries.

A command `shiftgen` is provided that generates SQL boilerplate to implement these interfaces.

```go
//go:generate shiftgen -inserter=create -updaters=pending,failed,completed -table=mysql_table_name
```

The `fsm` instance is then used by the business logic to drive the state machine.

```go
// Insert a new domain model (in the CREATED) state.
id, err := fsm.Insert(ctx, dbc, create{"user123",TypeDefault})

// Update it from CREATED to PENDING 
err = fsm.Update(ctx, dbc, CREATED, PENDING, pending{id})

// Update it from PENDING to COMPLETED 
err = fsm.Update(ctx, dbc, PENDING, COMPLETED, completed{id, "success!"})
``` 

> Note that the terms "state" and "status" are effective synonyms in this case. We found "state" to be an overtaxed term, so we use "status" in the code instead.

See [GoDoc](https://godoc.org/github.com/luno/shift) for details and this [example](shift_test.go).
                      
# Why?

Controlling domain model life cycle with Shift state machines provide the following benefits:
- Improved maintainability since everything is explicit.
- The code acts as documentation for the business logic.
- Decreased chance of inconsistent state.
- State transitions generate events, which other services subscribe to.
- Complex logic is broken down into discrete steps.
- Possible to avoid distributed transactions.

Shift state machines allow for robust fault tolerant systems that are easy to understand and maintain.
