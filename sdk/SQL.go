package sdk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidTransaction   = errors.New("Invalid transaction")
	ErrInvalidArgumentsScan = errors.New("Invalid arguments for scan")
	ErrNoColumnsReturned    = errors.New("No columns returned")
	ErrInvalidDatabase      = errors.New("Invalid database")
)

type (
	BeginTx interface {
		BeginTx(ctx context.Context, opts *sql.TxOptions) (tx *sql.Tx, err error)
	}
	ExecContext interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (res sql.Result, err error)
	}
	PingContext interface {
		PingContext(ctx context.Context) (err error)
	}
	PrepareContext interface {
		PrepareContext(ctx context.Context, query string) (stmt *sql.Stmt, err error)
	}
	QueryContext interface {
		QueryContext(ctx context.Context, query string, args ...interface{}) (rows *sql.Rows, err error)
	}
	QueryRowContext interface {
		QueryRowContext(ctx context.Context, query string, args ...interface{}) (row *sql.Row)
	}
)

// SQLConn is a common interface of *sql.DB and *sql.Conn.
type SQLConn interface {
	BeginTx
	io.Closer
	PingContext
	SQLTxConn
}

// SQLTxConn is a common interface of *sql.DB, *sql.Conn, and *sql.Tx.
type SQLTxConn interface {
	ExecContext
	PrepareContext
	QueryContext
	QueryRowContext
}

var (
	_ SQLConn   = (*sql.Conn)(nil)
	_ SQLConn   = (*sql.DB)(nil)
	_ SQLTxConn = (*sql.Tx)(nil)
)

const (
	_SELECT   = "SELECT"
	_INSERT   = "INSERT"
	_UPDATE   = "UPDATE"
	_DELETE   = "DELETE"
	_CREATE   = "CREATE"
	_ALTER    = "ALTER"
	_DROP     = "DROP"
	_USE      = "USE"
	_ADD      = "ADD"
	_EXEC     = "EXEC"
	_TRUNCATE = "TRUNCATE"
)

func SQLNoScan() interface{} { return new([]byte) }

type SQL struct{}

// BoxExec will wrap `ExecContext` so that we can Scan later
//
//	BoxExec(cmd.ExecContext(ctx, "..."))
//
// Scan the result of ExecContext that usually return numbers of rowsAffected
// and lastInsertID.
func (SQL) BoxExec(sqlResult sql.Result, err error) BoxExec { return boxExec{sqlResult, err} }

type BoxExec interface {
	Scan(rowsAffected *int, lastInsertID *int) (err error)
}

// BoxQuery will wrap `QueryContext` so that we can Scan later
//
//	BoxQuery(cmd.QueryContext(ctx, "..."))
//
// Scan all the rows and map it into []map[string]interface{} using column name
// as key in the map and then parse it into json format, after that unmarshal
// into designated dest.
func (SQL) BoxQuery(sqlRows *sql.Rows, err error) BoxQuery { return boxQuery{sqlRows, err} }

type BoxQuery interface {
	// Scan accept do, a func that accept `i int` as index and returns a List
	// of pointer.
	//  List == nil   // break the loop
	//  len(List) < 1 // skip the current loop
	//  len(List) > 0 // assign the pointer, must be same as the length of columns
	Scan(row func(i int) List) (err error)
}

// EndTx will end transaction with provided *sql.Tx and error. The tx argument
// should be valid, and then will check the err, if any error occurred, will
// commencing the ROLLBACK else will COMMIT the transaction.
//
//	txc := XSQLTxConn(db) // shared between *sql.Tx, *sql.DB and *sql.Conn
//	if tx, err := db.BeginTx(ctx, nil); err == nil && tx != nil {
//	  defer func() { err = xsql.EndTx(tx, err) }()
//	  txc = tx
//	}
func (SQL) EndTx(tx *sql.Tx, err error) error {
	if tx == nil {
		return fmt.Errorf("database: %w", ErrInvalidTransaction)
	}

	// if any error occurred, we try to rollback
	if msg := "rollback"; err != nil {
		if errR := tx.Rollback(); errR != nil {
			msg = fmt.Sprintf("%s failed: (%s)", msg, errR.Error())
		}

		return fmt.Errorf("database: %s because: %w", msg, err)
	}

	// we try to commit here
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("database: %w", err)
	}

	return nil
}

type boxExec struct {
	sqlResult sql.Result
	err       error
}

func (x boxExec) Scan(rowsAffected *int, lastInsertID *int) (err error) {
	err = x.err
	if err != nil {
		return fmt.Errorf("database: BoxExec: %w", err)
	}

	if x.sqlResult == nil {
		return fmt.Errorf("database: BoxExec: %w", ErrInvalidArgumentsScan)
	}

	if rowsAffected != nil {
		if n, err := x.sqlResult.RowsAffected(); err == nil {
			if n < 1 {
				return fmt.Errorf("database: BoxExec: %w", sql.ErrNoRows)
			}

			*rowsAffected = int(n)
		}
	}

	if lastInsertID != nil {
		if n, err := x.sqlResult.LastInsertId(); err == nil {
			*lastInsertID = int(n)
		}
	}

	return err
}

type boxQuery struct {
	sqlRows *sql.Rows
	err     error
}

func (x boxQuery) Scan(row func(i int) List) (err error) {
	err = x.err
	if err != nil {
		return err
	} else if x.sqlRows == nil {
		return fmt.Errorf("database: boxQuery: %w", sql.ErrNoRows)
	} else if err = x.sqlRows.Err(); err != nil {
		return err
	}
	defer x.sqlRows.Close()

	cols, err := x.sqlRows.Columns()
	if err != nil {
		return fmt.Errorf("database: boxQuery: %w", err)
	} else if len(cols) < 1 {
		return fmt.Errorf("database: boxQuery: %w", ErrNoColumnsReturned)
	}

	for i := 0; x.sqlRows.Next(); i++ {
		err = x.sqlRows.Err()
		if err != nil {
			return fmt.Errorf("database: boxQuery: %w", err)
		}

		dest := row(i)
		if dest == nil { // nil dest
			break
		} else if len(dest) < 1 { // empty dest
			continue
		} else if len(dest) != len(cols) { // diff dest & cols
			return fmt.Errorf("database: boxQuery: %w: [%d] columns on [%d] destinations", ErrInvalidArgumentsScan, len(cols), len(dest))
		}

		err = x.sqlRows.Scan(dest...) // scan into pointers
		if err != nil {
			return fmt.Errorf("database: boxQuery: %w", err)
		}
	}

	return err
}

// RemoveComment from sql command.
func (SQL) RemoveComment(query string) (query_ string) {
	commentStartIdx, replaces := -1, []string{}

	for i := range query {
		// we found sql comment
		if i-1 >= 0 && i+1 < len(query) && query[i] == '-' && query[i+1] == '-' && query[i-1] != '-' {
			commentStartIdx = i

			continue
		}

		if commentStartIdx > -1 && query[i] == '\n' {
			replaces = append(replaces, query[commentStartIdx:i])
		}
	}

	for _, v := range replaces {
		query = strings.Replace(query, v, "", 1)
	}

	return strings.TrimSpace(query)
}

// IsMultipleCommand is a naive implementation of checking multiple sql command.
func (SQL) IsMultipleCommand(query string) (ok bool) {
	validCount := 0

	for _, query := range strings.Split(query, ";") {
		query = strings.ToUpper(strings.TrimSpace(SQL{}.RemoveComment(query)))
		if (SQL{}).IsValidCommand(query) {
			validCount++
		}
	}

	return validCount > 1
}

// IsSELECTCommand only valid if starts with SELECT.
func (SQL) IsSELECTCommand(query string) (ok bool) {
	query = strings.ToUpper(strings.TrimSpace(SQL{}.RemoveComment(query)))
	for _, s := range []string{_SELECT} {
		ok = ok || strings.HasPrefix(query, s) || strings.Contains(query, s)
	}

	return ok
}

// IsDMLCommand only valid if starts with INSERT, UPDATE, DELETE.
func (SQL) IsDMLCommand(query string) (ok bool) {
	query = strings.ToUpper(strings.TrimSpace(SQL{}.RemoveComment(query)))
	for _, s := range []string{_INSERT, _UPDATE, _DELETE} {
		ok = ok || strings.HasPrefix(query, s)
	}

	return ok
}

// IsDDLCommand only valid if starts with CREATE, ALTER, DROP, USE, ADD, EXEC, TRUNCATE.
func (SQL) IsDDLCommand(query string) (ok bool) {
	query = strings.ToUpper(strings.TrimSpace(SQL{}.RemoveComment(query)))
	for _, s := range []string{_CREATE, _ALTER, _DROP, _USE, _ADD, _EXEC, _TRUNCATE} {
		ok = ok || strings.HasPrefix(query, s)
	}

	return ok
}

func (SQL) IsValidCommand(query string) (ok bool) {
	return SQL{}.IsSELECTCommand(query) || SQL{}.IsDMLCommand(query) || SQL{}.IsDDLCommand(query)
}

// SetupOrTeardown will execute multiple queries and useful in SETUP/TEARDOWN
// phase of Behavior Driven Development (BDD). BDD approach is useful to make sure
// that the SQL Query should satisfied the syntax and reduce integration issues.
func (SQL) SetupOrTeardown(ctx context.Context, conn ExecContext, queries ...string) error {
	for _, q := range queries {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return err
		}
	}

	return nil
}
