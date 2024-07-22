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
	// FIXME separate query building from execution
	// FIXME accumulate values and use ? instead of hardcoding

	columns := strings.Join(append(spec.GroupByMetrics, "count(1) '#reqs'"), ",")
	var groupByExpression string
	if len(spec.GroupByMetrics) > 0 {
		groupByExpression = fmt.Sprintf("GROUP BY %s", strings.Join(spec.GroupByMetrics, ","))
	}

	whereConditions := make([]string, 0)
	for column, values := range spec.Where {
		columnConditions := make([]string, len(values))
		for i, value := range values {
			columnConditions[i] = fmt.Sprintf("%s=%s", column, value)
		}
		whereConditions = append(whereConditions, fmt.Sprintf("(%s)", strings.Join(columnConditions, " OR ")))
	}

	var whereExpression string
	if len(whereConditions) > 0 {
		whereExpression = " AND " + strings.Join(whereConditions, " AND ")
	}

	// FIXME handle WHERE conditions
	queryString := fmt.Sprintf(`SELECT %s FROM access_logs
WHERE time > ? AND time < ? AND status <> 304 %s %s
ORDER BY count(1) DESC
LIMIT %d
`, columns, whereExpression, groupByExpression, spec.Limit)

	log.Printf("query: %s\n", queryString)

	return db.Query(
		queryString,
		spec.TimeSince,
		spec.TimeUntil,
	)
}
