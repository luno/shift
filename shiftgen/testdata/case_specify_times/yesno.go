package case_specify_times

import (
	"database/sql"
	"database/sql/driver"
)

const (
	Unknown = 0
	Yes     = 1
	No      = 2
	Maybe   = 3
)

type YesNoMaybe int

func (v *YesNoMaybe) Scan(src interface{}) error {
	var s sql.NullString
	if err := s.Scan(src); err != nil {
		return err
	}
	if !s.Valid {
		*v = Unknown
		return nil
	}
	switch s.String {
	case "yes":
		*v = Yes
	case "no":
		*v = No
	case "maybe":
		*v = Maybe
	default:
		*v = Unknown
	}
	return nil
}

func (v YesNoMaybe) Value() (driver.Value, error) {
	switch v {
	case Yes:
		return "yes", nil
	case No:
		return "no", nil
	case Maybe:
		return "maybe", nil
	default:
		return "", nil
	}
}
