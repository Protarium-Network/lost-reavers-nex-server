// Package database owns the Postgres connection, the S3-compatible object
// store connection, and every table specific to this server. Matchmaking and
// NEX unique IDs get their own schema from nex-protocols-common-go directly.
package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"lost-reavers/globals"
)

// Postgres is the shared connection pool.
var Postgres *sql.DB

// Connect opens the pool, verifies it is reachable and creates this server's
// own tables. It returns an error rather than exiting so main can report it.
func Connect(uri string) error {
	var err error

	Postgres, err = sql.Open("postgres", uri)
	if err != nil {
		return fmt.Errorf("could not open Postgres connection: %w", err)
	}

	if err := Postgres.Ping(); err != nil {
		return fmt.Errorf("could not reach Postgres: %w", err)
	}

	globals.Postgres = Postgres

	if err := initSchema(); err != nil {
		return err
	}

	globals.Logger.Success("Connected to Postgres")

	return nil
}

func initSchema() error {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS ranking`,

		`CREATE TABLE IF NOT EXISTS ranking.scores (
			pid         numeric(20) NOT NULL,
			unique_id   numeric(20) NOT NULL DEFAULT 0,
			category    bigint      NOT NULL,
			score       bigint      NOT NULL,
			order_by    smallint    NOT NULL DEFAULT 0,
			groups      bytea       NOT NULL DEFAULT ''::bytea,
			param       numeric(20) NOT NULL DEFAULT 0,
			update_time timestamp   NOT NULL DEFAULT (now() at time zone 'utc'),
			PRIMARY KEY (pid, unique_id, category)
		)`,

		`CREATE INDEX IF NOT EXISTS scores_category_score_idx ON ranking.scores (category, score)`,

		`CREATE TABLE IF NOT EXISTS ranking.common_data (
			unique_id numeric(20) NOT NULL,
			pid       numeric(20) NOT NULL,
			data      bytea       NOT NULL,
			PRIMARY KEY (unique_id, pid)
		)`,

		// * Created early: nex-protocols-common-go also claims this schema when
		// * the matchmaking manager attaches, later. CREATE SCHEMA IF NOT EXISTS
		// * is idempotent, so the block list can live beside the gatherings it
		// * filters without caring which one runs first.
		`CREATE SCHEMA IF NOT EXISTS matchmaking`,

		`CREATE TABLE IF NOT EXISTS matchmaking.block_list (
			pid         numeric(20) NOT NULL,
			blocked_pid numeric(20) NOT NULL,
			PRIMARY KEY (pid, blocked_pid)
		)`,
	}

	for _, statement := range statements {
		if _, err := Postgres.Exec(statement); err != nil {
			return fmt.Errorf("failed to create ranking/matchmaking tables: %w", err)
		}
	}

	if err := initDataStoreSchema(); err != nil {
		return err
	}

	return nil
}
