package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
	"time"
)

type RequestCountSpec struct {
	GroupByMetrics []string
	TimeSince      time.Time
	TimeUntil      time.Time
	Limit          int
	Where          map[string][]string
}

type dbSession struct {
	db         *sql.DB
	columns    []string
	insertTx   *sql.Tx
	insertStmt *sql.Stmt
}

const DB_DATE_LAYOUT = "2006-01-02 15:04:05-07:00"

// Open or create the database at the given path.
func InitDB(dbPath string, fields []*LogField) (*dbSession, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("PRAGMA journal_mode=memory;")
	if err != nil {
		return nil, err
	}

	// TODO consider adding indexes according to expected queries

	var columnSpecs string
	columns := make([]string, len(fields))
	for i, field := range fields {
		columns[i] = field.ColumnName
		columnSpecs += fmt.Sprintf("%s %s,\n", field.ColumnName, field.ColumnSpec)
	}

	sqlStmt := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS access_logs (
			id 				INTEGER NOT NULL PRIMARY KEY,
			%s
			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`, columnSpecs)

	_, err = db.Exec(sqlStmt)
	return &dbSession{db: db, columns: columns}, err
}

func (dbs *dbSession) Close() {
	dbs.db.Close()
}

// Prepare a transaction to insert a new batch of log entries, returning the time of the last seen log entry.
func (dbs *dbSession) PrepareForUpdate() (*time.Time, error) {
	// we want to avoid processed files that were already processed in the past.  but we still want to add new log entries
	// from the most recent files, which may have been extended since we last saw them.
	// Since there is no "uniqueness" in logs (even the same ip can make the same request at the same second ---I checked),
	// I remove the entries with the highest timestamp, and load everything up until including that timestamp but not older.
	// The assumption is that any processing was completely finished, not interrupted.

	var lastSeenTimeStr string
	var lastSeemTime *time.Time
	// this query error is acceptable in case of db not exists or empty
	if err := dbs.db.QueryRow("SELECT max(time) FROM access_logs").Scan(&lastSeenTimeStr); err == nil {
		_, err := dbs.db.Exec("DELETE FROM access_logs WHERE time = ?", lastSeenTimeStr)
		if err != nil {
			return nil, err
		}

		t, _ := time.Parse(DB_DATE_LAYOUT, lastSeenTimeStr)
		lastSeemTime = &t
	}

	// prepare transaction for log inserts
	tx, err := dbs.db.Begin()
	if err != nil {
		return nil, err
	}
	dbs.insertTx = tx

	insertValuePlaceholder := strings.TrimSuffix(strings.Repeat("?,", len(dbs.columns)), ",")
	insertStmt, err := dbs.insertTx.Prepare(fmt.Sprintf("INSERT INTO access_logs(%s) values(%s);", strings.Join(dbs.columns, ","), insertValuePlaceholder))
	if err != nil {
		return nil, err
	}
	dbs.insertStmt = insertStmt
	return lastSeemTime, nil
}

func (dbs *dbSession) AddLogEntry(values ...any) error {
	_, err := dbs.insertStmt.Exec(values...)
	return err
}

// If the given processing `err` is nil, commit the log insertion transaction,
// Otherwise roll it back and return the error.
func (dbs *dbSession) FinishUpdate(err error) error {
	tx := dbs.insertTx
	dbs.insertTx = nil
	dbs.insertStmt = nil

	if err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// Build a query from the spec and execute it, returning the results as stringified values.
func (dbs *dbSession) QueryTop(spec *RequestCountSpec) ([]string, [][]string, error) {
	queryString, queryArgs := spec.buildQuery()

	rows, err := dbs.db.Query(queryString, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	values := make([]interface{}, len(columns))
	for i := range len(columns) {
		values[i] = new(sql.RawBytes)
	}

	var results [][]string

	for rows.Next() {
		err = rows.Scan(values...)
		if err != nil {
			return nil, nil, err
		}
		strValues := make([]string, len(values))
		for i, value := range values {
			strValues[i] = string(*value.(*sql.RawBytes))
		}
		results = append(results, strValues)
	}
	return columns, results, rows.Err()
}

// Turn the request count specification into an SQL query.
func (spec *RequestCountSpec) buildQuery() (string, []any) {
	queryArgs := []any{}

	whereExpression := "WHERE time > ? AND time < ? "
	queryArgs = append(queryArgs, spec.TimeSince, spec.TimeUntil)
	for column, values := range spec.Where {
		whereExpression += "AND ("

		for i, value := range values {
			whereExpression += column
			if strings.ContainsRune(value, '%') {
				if strings.HasPrefix(value, "!") {
					value = strings.TrimPrefix(value, "!")
					whereExpression += " NOT LIKE ?"
				} else {
					whereExpression += " LIKE ?"
				}
			} else {
				if strings.HasPrefix(value, "!") {
					value = strings.TrimPrefix(value, "!")
					whereExpression += " <> ?"
				} else {
					whereExpression += " = ?"
				}
			}
			queryArgs = append(queryArgs, value)
			if i < len(values)-1 {
				whereExpression += " OR "
			}
		}
		whereExpression += ") "
	}

	columns := strings.Join(append(spec.GroupByMetrics, "count(1) '#reqs'"), ",")
	var groupByExpression string
	if len(spec.GroupByMetrics) > 0 {
		groupByExpression = "GROUP BY"
		for i := range len(spec.GroupByMetrics) {
			groupByExpression += fmt.Sprintf(" %d", i+1)
			if i < len(spec.GroupByMetrics)-1 {
				groupByExpression += ","
			}
		}
	}

	queryString := fmt.Sprintf(
		"SELECT %s FROM access_logs %s %s ORDER BY count(1) DESC LIMIT %d",
		columns,
		whereExpression,
		groupByExpression,
		spec.Limit, // the limit clause can't be "?"
	)

	log.Printf("query: %s %s\n", queryString, queryArgs)

	return queryString, queryArgs
}
