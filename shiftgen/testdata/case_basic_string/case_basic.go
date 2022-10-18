package case_basic

import "time"

type insert struct {
	ID          string
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
}

type update struct {
	ID     string
	Name   string
	Amount Currency
}

type complete struct {
	ID string
}
