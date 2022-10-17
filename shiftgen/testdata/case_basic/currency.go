package case_basic

import (
	"database/sql"
	"database/sql/driver"
	"strconv"
)

// Currency is a custom "currency" type stored a string in the DB.
type Currency struct {
	Valid  bool
	Amount int64
}

func (c *Currency) Scan(src interface{}) error {
	var s sql.NullString
	if err := s.Scan(src); err != nil {
		return err
	}
	if !s.Valid {
		*c = Currency{
			Valid:  false,
			Amount: 0,
		}
		return nil
	}
	i, err := strconv.ParseInt(s.String, 10, 64)
	if err != nil {
		return err
	}
	*c = Currency{
		Valid:  true,
		Amount: i,
	}
	return nil
}

func (c Currency) Value() (driver.Value, error) {
	return strconv.FormatInt(c.Amount, 10), nil
}
