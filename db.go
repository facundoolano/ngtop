package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// FIXME or in its own mod, and turn it into the "lingua franca" of the cli
// eg. from cli, as sql, as table
type RequestCountSpec struct {
	GroupByMetrics []string
	TimeSince      time.Time
	TimeUntil      time.Time
	Limit          int
	Where          map[string][]string
}

func (spec *RequestCountSpec) Exec(db *sql.DB) (*sql.Rows, error) {
	columns := strings.Join(append(spec.GroupByMetrics, "count(1) '#reqs'"), ",")
	var groupBy string
	if len(spec.GroupByMetrics) > 0 {
		groupBy = fmt.Sprintf("GROUP BY %s", strings.Join(spec.GroupByMetrics, ","))
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
