package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// FIXME this should probably go to the db file
type RequestCountQuery struct {
	ColumnGroup []string
	TimeSince   time.Time
	TimeUntil   time.Time
	Limit       int
	Where       string
}

func (spec *RequestCountQuery) Exec(db *sql.DB) (*sql.Rows, error) {
	columns := strings.Join(append(spec.ColumnGroup, "count(1)"), ",")
	var groupBy string
	if len(spec.ColumnGroup) > 0 {
		groupBy = fmt.Sprintf("GROUP BY %s", strings.Join(spec.ColumnGroup, ","))
	}

	// FIXME handle WHERE conditions
	queryString := fmt.Sprintf(`SELECT %s FROM access_logs
WHERE time > ? AND time < ? AND status <> 304 %s
ORDER BY count(1) DESC
LIMIT %d
`, columns, groupBy, spec.Limit)

	log.Printf("query: %s\n", queryString)

	return db.Query(
		queryString,
		spec.TimeSince,
		spec.TimeUntil,
		spec.Limit,
	)
}
