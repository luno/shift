package case_specify_times

import (
	"database/sql"
	"time"
)

type iFoo struct {
	I1        int64
	I2        string
	I3        time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type uFoo struct {
	ID        int64
	U1        bool
	U2        YesNoMaybe
	U3        sql.NullTime
	U4        sql.NullString
	U5        []byte
	UpdatedAt time.Time
}
