package main

import (
	"database/sql"
	"fmt"
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

// FIXME separate query building from execution
func (spec *RequestCountSpec) Exec(db *sql.DB) (*sql.Rows, error) {
	queryArgs := []any{}

	whereExpression := "WHERE time > ? AND time < ? "
	queryArgs = append(queryArgs, spec.TimeSince, spec.TimeUntil)
	for column, values := range spec.Where {
		whereExpression += "AND ("

		for i, value := range values {
			whereExpression += column
			if strings.ContainsRune(value, '%') {
				whereExpression += " LIKE ?"
			} else {
				whereExpression += " = ?"
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

	// FIXME handle WHERE conditions
	queryString := fmt.Sprintf(
		"SELECT %s FROM access_logs %s %s ORDER BY count(1) DESC LIMIT %d",
		columns,
		whereExpression,
		groupByExpression,
		spec.Limit, // the limit clause can't be "?"
	)

	log.Printf("query: %s %s\n", queryString, queryArgs)

	return db.Query(
		queryString,
		queryArgs...,
	)
}
