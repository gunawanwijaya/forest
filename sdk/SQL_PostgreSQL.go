package sdk

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/lib/pq"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// nolint: gochecknoglobals
var (
	ErrMultipleCommands = errors.New("Multiple commands")
	ErrInvalidCommand   = errors.New("Invalid command")
)

func PostgreSQLArray(array interface{}) interface {
	driver.Valuer
	sql.Scanner
} {
	return struct {
		driver.Valuer
		sql.Scanner
	}{
		v{pq.Array(array)},
		s{pgdialect.Array(array)},
	}
}

type v struct{ driver.Valuer }

func (v v) Value() (driver.Value, error) { return v.Valuer.Value() }

type s struct{ sql.Scanner }

func (s s) Scan(src interface{}) error { return s.Scanner.Scan(src) }

// -----------------------------------------------------------------------------
// RoundRobin
// -----------------------------------------------------------------------------

// sqlRoundRobin consists of multiple *sql.DB connection, assuming the first element
// of sources is a READ/WRITE access and the rest is READ-ONLY access.
type sqlRoundRobin struct {
	SQL
	conns []SQLConn
	index sqlRoundRobinIndex
}

type sqlRoundRobinIndex struct {
	int
	*sync.Mutex
}

// OpenWithDSN will open connection from the given dsn string with URL format, note
// that any error when opening the database should result in a panic.
func (SQL) OpenWithDSN(ctx context.Context, dsn string) (conn *sql.DB, err error) {
	driverName := strings.Split(dsn+"://", "://")[0]
	switch driverName {
	case "postgres", "postgresql":
		conn = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		if conn == nil {
			return nil, ErrInvalidDatabase
		}
	default:
		conn, err = sql.Open(driverName, dsn)
		if err != nil {
			return nil, err
		} else if conn == nil {
			return nil, ErrInvalidDatabase
		}
	}

	return conn, nil
}

// NewRoundRobin will reduce multiple connections into one with RoundRobin style.
func (SQL) NewRoundRobin(ctx context.Context, conns ...SQLConn) SQLConn {
	if l := len(conns); l < 1 {
		// this is because we expect that `Add` should connect to at least one
		// database connection to act as primary conn
		panic("empty database")
	}

	return &sqlRoundRobin{SQL{}, conns, sqlRoundRobinIndex{0, new(sync.Mutex)}}
}

// BeginTx READ+WRITE database.
func (rr *sqlRoundRobin) BeginTx(ctx context.Context, opts *sql.TxOptions) (tx *sql.Tx, err error) {
	conn, err := rr.get(0)
	if err != nil {
		return nil, err
	}

	return conn.BeginTx(ctx, opts)
}

// Close all databases.
func (rr *sqlRoundRobin) Close() (err error) {
	errs := new(ListError)
	for i := range rr.conns {
		errs = errs.Add(rr.conns[i].Close())
	}

	return errs.Err()
}

// PingContext all databases.
func (rr *sqlRoundRobin) PingContext(ctx context.Context) (err error) {
	errs := new(ListError)
	for i := range rr.conns {
		errs = errs.Add(rr.conns[i].PingContext(ctx))
	}

	return errs.Err()
}

// PrepareContext valid queries are DDL, DML & SELECT.
func (rr *sqlRoundRobin) PrepareContext(ctx context.Context, query string) (stmt *sql.Stmt, err error) {
	query = rr.RemoveComment(query)
	conn := SQLConn(nil)

	if rr.IsMultipleCommand(query) {
		return nil, fmt.Errorf("database: %w", ErrMultipleCommands)
	} else if !rr.IsValidCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if rr.IsDDLCommand(query) {
		conn, err = rr.get(0)
	} else if rr.IsDMLCommand(query) {
		conn, err = rr.get(0)
	} else if rr.IsSELECTCommand(query) {
		conn, err = rr.get(-2)
	} else {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if err != nil {
		return nil, err
	}

	return conn.PrepareContext(ctx, query)
}

// ExecContext valid queries are DDL & DML.
func (rr *sqlRoundRobin) ExecContext(ctx context.Context, query string, args ...interface{}) (res sql.Result, err error) {
	query = rr.RemoveComment(query)
	conn := SQLConn(nil)

	if rr.IsMultipleCommand(query) {
		return nil, fmt.Errorf("database: %w", ErrMultipleCommands)
	} else if !rr.IsValidCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if rr.IsDDLCommand(query) {
		conn, err = rr.get(0)
	} else if rr.IsDMLCommand(query) {
		conn, err = rr.get(0)
	} else if rr.IsSELECTCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	} else {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if err != nil {
		return nil, err
	}

	return conn.ExecContext(ctx, query, args...)
}

// QueryContext valid queries are SELECT.
func (rr *sqlRoundRobin) QueryContext(ctx context.Context, query string, args ...interface{}) (rows *sql.Rows, err error) {
	query = rr.RemoveComment(query)
	conn := SQLConn(nil)

	if rr.IsMultipleCommand(query) {
		return nil, fmt.Errorf("database: %w", ErrMultipleCommands)
	} else if !rr.IsValidCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if rr.IsSELECTCommand(query) {
		conn, err = rr.get(-2)
	} else if rr.IsDDLCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	} else if rr.IsDMLCommand(query) {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	} else {
		return nil, fmt.Errorf("database: %w: %q", ErrInvalidCommand, query)
	}

	if err != nil {
		return nil, err
	}

	return conn.QueryContext(ctx, query, args...)
}

// QueryRowContext valid queries are SELECT.
func (rr *sqlRoundRobin) QueryRowContext(ctx context.Context, query string, args ...interface{}) (row *sql.Row) {
	query = rr.RemoveComment(query)
	conn := SQLConn(nil)

	if rr.IsMultipleCommand(query) {
		return nil
	} else if !rr.IsValidCommand(query) {
		return nil
	}

	if rr.IsSELECTCommand(query) {
		conn, _ = rr.get(-2)
	} else if rr.IsDDLCommand(query) {
		return nil
	} else if rr.IsDMLCommand(query) {
		return nil
	} else {
		return nil
	}

	return conn.QueryRowContext(ctx, query, args...)
}

// get will return a new Conn that balanced using roundRobin
//
//	rr.get(0)    -> direct READ+WRITE
//	rr.get(1..n) -> direct READ-ONLY
//	rr.get(-1)   -> roundRobin READ+WRITE and READ-ONLY
//	rr.get(-2)   -> roundRobin READ-ONLY
func (rr *sqlRoundRobin) get(i int) (SQLConn, error) {
	l := len(rr.conns)

	switch {
	case l == 1: // only one
		rr.index.int = 0
	case i >= 0 && l > i: // direct
		rr.index.int = i
	case (i == -1 || i == -2) && l > 1: // roundRobin
		rr.index.Lock()
		if rr.index.int++; rr.index.int >= l {
			switch i {
			case -1: // roundRobin READ/WRITE and READ-ONLY
				rr.index.int = 0
			case -2: // roundRobin READ-ONLY
				rr.index.int = 1
			}
		}
		rr.index.Unlock()
	default:
		return nil, &SQLRoundRobinError{l, i}
	}

	return rr.conns[rr.index.int], nil
}

// SQLRoundRobinError reporting issue when getting from set of Conn from SQLRoundRobin.
type SQLRoundRobinError struct{ Total, Index int }

func (e *SQLRoundRobinError) Error() string {
	return fmt.Sprintf("database: Unable to connect to database on index %d with total %d element(s).", e.Index, e.Total)
}
