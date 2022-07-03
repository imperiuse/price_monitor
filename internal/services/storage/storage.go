package storage

import (
	"context"
	"errors"
	"github.com/Masterminds/squirrel"
	"github.com/imperiuse/golib/db"
	"github.com/jmoiron/sqlx"

	"github.com/imperiuse/price_monitor/internal/services/storage/model"
)

type (
	CustomDbConfig interface {
		db.Config
		CustomConfig() Config
	}

	// Config - config of storage.
	Config struct {
		// todo in real world array of hosts or slaves or other // can use Consul hook
		Host                           string
		Port                           int
		Username                       string
		Password                       string
		Database                       string
		IsEnableValidationForRepoNames bool   `yaml:"IsEnableValidationForRepoNames"`
		IsEnableReposStructCache       bool   `yaml:"IsEnableReposStructCache"`
		Placeholder                    string `yaml:"placeholder"`
		MaxTryConnect                  int    `yaml:"maxTryConnect"`
		TimeoutTryConnect              string `yaml:"timeoutTryConnect"`
		Options                        Options
	}

	// Options - options config.
	Options map[string]any
)

// Realization golib.db.Config interface

func (c Config) IsEnableValidationRepoNames() bool {
	return c.IsEnableValidationForRepoNames
}

func (c Config) IsEnableReposCache() bool {
	return c.IsEnableReposStructCache
}

func (c Config) PlaceholderFormat() squirrel.PlaceholderFormat {
	switch c.Placeholder {
	case "$":
		return squirrel.Dollar // $1, $2, etc // Postgres
	case "?":
		return squirrel.Question
	case "@":
		return squirrel.AtP
	default:
		return squirrel.Dollar
	}
}

func (c Config) CustomConfig() Config {
	return c
}

var Select = squirrel.Select

var ErrNotInserted = errors.New("not inserted record to db")

// NB! moq - useful param -skip-ensure
//go:generate moq  -out ../../mocks/mock_storage.go -skip-ensure -pkg mocks . Storage Connector Repository
type (
	// Query - sql query string.
	Query = db.Query
	// Column - sql column name.
	Column = db.Column
	// Argument - anything ibj which have Valuer and Scan.
	Argument = db.Argument
	// Alias - alias for table.
	Alias = db.Alias
	// Join - join part of query.
	Join = db.Join
	// DTO - data table object
	DTO = db.DTO

	// SelectBuilder alias of squirrel.SelectBuilder.
	SelectBuilder = db.SelectBuilder

	// Eq - alias squirell.Eq
	Eq = squirrel.Eq
	// And - and pred.
	And = squirrel.And
	// Lt - and pred.
	Lt = squirrel.Lt
	// Gt - and pred.
	Gt = squirrel.Gt

	// CursorPaginationParams alias of CursorPaginationParams.
	CursorPaginationParams = db.CursorPaginationParams

	// Storage - instanced by Config type Storage generic interface
	Storage = _Storage[CustomDbConfig]
	// Connector - instanced by Config type Connector generic interface
	Connector = db.Connector[CustomDbConfig]

	// Repository - repository
	Repository = db.Repository

	// Storage - general storage interface.
	_Storage[C CustomDbConfig] interface {
		Config() C

		Connect(C) error

		Connector() db.Connector[C]
		PureSqlxDB() *sqlx.DB

		// TODO Master/Slave other variants

		Refresh(context.Context, []model.Table) error

		Close()
	}
)
