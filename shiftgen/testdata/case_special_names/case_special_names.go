package case_special_names

import (
	"time"
)

type 类型 struct {
	Name string
}

type 변수 struct {
	ID        int64
	Name      string
	UpdatedAt time.Time
}

type エラー struct {
	ID        int64
	Surname   string
	UpdatedAt time.Time
}
