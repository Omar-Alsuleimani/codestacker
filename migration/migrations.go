package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//go:embed schema/*.sql
var schema embed.FS

// LogMode indicated the log mode when running migrations
type LogMode string

const (
	LogModeDebug LogMode = "debug"
	LogModeError LogMode = "error"
)

type migrateLogger struct {
	*logrus.Logger
}

func (m *migrateLogger) Verbose() bool {
	return true
}

// Migrate creates a db connection then migrate
func Migrate(connStr string, logMode LogMode) error {
	logger := logrus.New()

	sourceDriver, err := iofs.New(schema, "schema")
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	defer db.Close()

	databaseDriver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	migrateInstance, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		"postgres", databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if logMode == LogModeDebug {
		migrateInstance.Log = &migrateLogger{logger}
	}

	err = migrateInstance.Up()
	if err != nil {
		version, dirty, versionErr := migrateInstance.Version()
		if versionErr != nil {
			return fmt.Errorf("failed to get version: %w", versionErr)
		}

		err = errors.Cause(err)
		if err == migrate.ErrNoChange {
			logger.Infof("db is at version (%d). No database migrations to apply", version)
			return nil
		}

		logger.Infof("current version: %d, dirty: %v", version, dirty)

		if dirty {
			err := migrateInstance.Force(int(version - 1))
			if err != nil {
				return fmt.Errorf("failed to force migration version: %w", err)
			}

			logger.Infof("forcing back to version (%d)", version-1)
		}

		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if logMode == LogModeDebug {
		logger.Info("successfully applied migration")
	}

	return nil
}
