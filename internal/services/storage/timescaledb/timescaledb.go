package timescaledb

import (
	"context"
	"fmt"
	"time"

	"github.com/imperiuse/golib/db"
	"github.com/imperiuse/golib/db/connector"
	"github.com/imperiuse/golib/reflect/orm"
	"github.com/jmoiron/sqlx"
	"github.com/mitchellh/mapstructure"

	"github.com/imperiuse/price_monitor/internal/logger"
	"github.com/imperiuse/price_monitor/internal/logger/field"
	"github.com/imperiuse/price_monitor/internal/services/storage"
	"github.com/imperiuse/price_monitor/internal/services/storage/model"
)

type (
	storagePostgres[C storage.CustomDbConfig] struct {
		logger *logger.Logger

		cfg       C
		pgOptions postgresConfig

		connector db.Connector[C]
		purSqlxDB *sqlx.DB

		// TODO we can use this master slave approach too
		//masterSqlxDB *sqlx.DB
		//slavesSqlxDB []*sqlx.DB
		//masterConnector  db.Connector[C]
		//slavesConnectors []db.Connector[C]
	}

	postgresConfig struct {
		MaxLifeTime int `yaml:"max_life_time"` // in seconds
		MaxIdleConn int `yaml:"max_idle_conn"`
		MaxOpenConn int `yaml:"max_open_conn"`
	}
)

// New create new storagePostgres which impl. storage.Storage[C].
func New[C storage.CustomDbConfig](config C, logger *logger.Logger) (storage.Storage, error) {
	orm.InitMetaTagInfoCache(model.AllDTO...) // for cache info about dto obj

	s := &storagePostgres[storage.CustomDbConfig]{
		logger: logger,
	}

	connectDuration, err := time.ParseDuration(config.CustomConfig().TimeoutTryConnect)
	if err != nil {
		return nil, fmt.Errorf("err time.ParseDuration(config.CustomConfig().TimeoutTryReconnect): %w", err)
	}

	// TODO can use backoff mechanism  (google backoff)
	for attempt := 0; attempt < config.CustomConfig().MaxTryConnect; attempt++ {
		if err = s.Connect(config); err == nil {
			return s, nil
		}

		time.Sleep(connectDuration)
	}

	return nil, err
}

// Config - config
func (s *storagePostgres[C]) Config() C {
	return s.cfg
}

// Connect - connect stuff
func (s *storagePostgres[C]) Connect(cfg C) error {
	s.cfg = cfg

	s.pgOptions = postgresConfig{}
	err := mapstructure.Decode(s.cfg.CustomConfig().Options, &s.pgOptions)
	if err != nil {
		return fmt.Errorf("could not parse Custom Options for Storage: %w", err)
	}

	s.purSqlxDB, err = s.createMasterConn()
	if err != nil {
		return fmt.Errorf("can't create master db conn: %w", err)
	}

	s.logger.Info("Successfully connect to Master db",
		field.String("host", s.cfg.CustomConfig().Host), field.Int("port", s.cfg.CustomConfig().Port))

	s.connector = connector.New[C](s.cfg, s.logger, s.purSqlxDB)

	return nil
}

// createMasterConn - create dsn for master db
func (s *storagePostgres[C]) createMasterConn() (*sqlx.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		s.cfg.CustomConfig().Username,
		s.cfg.CustomConfig().Password,
		s.cfg.CustomConfig().Host,
		s.cfg.CustomConfig().Port,
		s.cfg.CustomConfig().Database,
	)

	return s.createConn(dsn)
}

// createConn - create sqlx DB connection
func (s *storagePostgres[C]) createConn(dsn string) (*sqlx.DB, error) {
	dbConn, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlx.Connect: %w", err)
	}

	dbConn.SetConnMaxLifetime(time.Second * time.Duration(s.pgOptions.MaxLifeTime))
	dbConn.SetMaxIdleConns(s.pgOptions.MaxIdleConn)
	dbConn.SetMaxOpenConns(s.pgOptions.MaxOpenConn)

	return dbConn, nil
}

// Connector - return master db.Connector[C]
func (s *storagePostgres[C]) Connector() db.Connector[C] {
	return s.connector
}

// PureSqlxDB - return pure sqlx DB obj
func (s *storagePostgres[C]) PureSqlxDB() *sqlx.DB {
	return s.purSqlxDB
}

// Close connection to all DBs
func (s *storagePostgres[C]) Close() {
	s.logger.Warn("Close connection for Storage Postgres")

	err := s.purSqlxDB.Close()
	logger.LogIfError(s.logger, "Err while close master connection", err)

	return
}

// Refresh - for dev db, refresh tables of db (truncate under the hood).
func (s *storagePostgres[C]) Refresh(ctx context.Context, tables []model.Table) error {
	s.logger.Sugar().Info("Refresh DB. Truncate tables: ", tables, " Set seq to 1;")

	for _, tableName := range tables {
		_, err := s.purSqlxDB.ExecContext(ctx,
			fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;", tableName))
		if err != nil {
			return fmt.Errorf("s.db.ExecContext: %w", err)
		}
	}

	return nil
}
