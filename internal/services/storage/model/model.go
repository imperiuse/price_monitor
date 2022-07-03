package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/imperiuse/golib/db"
	"github.com/imperiuse/golib/reflect/orm"
)

var (
	AllDTO = []any{
		Currency{},
		Monitoring{},
		Price{},
	}
)

const (
	BtcUsd CurrencyCode = "BTCUSD"
)

var PriceTableNameGetterFunc = func(code CurrencyCode) Table {
	return fmt.Sprintf("%s_prices", strings.ToLower(code))
}

type (
	// Identity - identity
	Identity = int64

	// Table in DB
	Table = string

	// CurrencyCode - currency code // varchar[10]
	CurrencyCode = string

	// Currency - dto for currencies obj
	Currency struct {
		ID           Identity     `db:"id" orm_use_in:"select" json:"id"`
		CurrencyCode CurrencyCode `db:"currency_code" orm_use_in:"select,create" json:"currency_code"`
		_            any          `orm_table_name:"currencies"`
	}

	// basePrice - dto for <CURRENCY_CODE>_prices tables
	Price struct {
		Time  time.Time `db:"time" orm_use_in:"select,create" json:"time"`
		Price float64   `db:"price" orm_use_in:"select,create" json:"price"`
	}

	// Monitoring - dto for monitoring price obj
	Monitoring struct {
		ID         Identity  `db:"id" orm_use_in:"select" json:"id"`
		CreatedAt  time.Time `db:"created_at" orm_use_in:"select,create" json:"created_at"`
		StartedAt  time.Time `db:"started_at" orm_use_in:"select,create" json:"started_at"`
		ExpiredAt  time.Time `db:"expired_at" orm_use_in:"select,create" json:"expired_at"`
		Frequency  string    `db:"frequency" orm_use_in:"select,create" json:"frequency"`
		CurrencyID Identity  `db:"currency_id" orm_use_in:"select,create" json:"currency_id"`
		_          any       `orm_table_name:"monitorings"`
	}
)

// impl db.DTO methods (this part can be automatized by go: generators)

func (m Monitoring) Repo() db.Table {
	return orm.GetTableName(m) // cached
}

func (m Monitoring) Identity() db.ID {
	return m.ID
}

func (c Currency) Repo() db.Table {
	return orm.GetTableName(c) // cached
}

func (c Currency) Identity() db.ID {
	return c.ID
}
