package case_basic

import "time"

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
