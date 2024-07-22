package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

// FIXME move this to main
var cli struct {
	Fields []string `arg:"" optional:"" help:"TODO"`
	Since  string   `short:"s" default:"1h" help:"TODO"`
	Until  string   `short:"u" default:"now"  help:"TODO"`
	Limit  int      `short:"l" default:"5" help:"TODO"`
	Where  []string `short:"w" optional:"" help:"TODO"`
}

var FIELD_NAMES = map[string]string{
	"user_agent": "user_agent_raw",
	"useragent":  "user_agent_raw",
	"ua":         "user_agent_raw",
	"request":    "request_raw",
	"bytes":      "bytes_sent",
	"bytes_sent": "bytes_sent",
	"path":       "path",
	"url":        "path",
	"ip":         "ip",
	"referer":    "referer",
	"status":     "status",
	"method":     "method",
}

func querySpecFromCLI() RequestCountSpec {
	kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.Vars{"version": "ngtop v0.1.0"},
	)

	since, err := parseDuration(cli.Since)
	checkError(err)
	until, err := parseDuration(cli.Until)
	checkError(err)

	columns := make([]string, len(cli.Fields))
	for i, field := range cli.Fields {
		if value, found := FIELD_NAMES[field]; !found {
			// FIXME return error instead
			checkError(fmt.Errorf("unknown field name %s", field))
		} else {
			columns[i] = value
		}
	}

	whereConditions, err := resolveWhereConditions(cli.Where)
	checkError(err)
	return RequestCountSpec{
		GroupByMetrics: columns,
		TimeSince:      since,
		TimeUntil:      until,
		Limit:          cli.Limit,
		Where:          whereConditions,
	}
}

func resolveWhereConditions(clauses []string) (map[string][]string, error) {
	conditions := make(map[string][]string)

	for _, clause := range clauses {
		keyvalue := strings.Split(clause, "=")
		if len(keyvalue) != 2 {
			return nil, fmt.Errorf("invalid where expression %s", clause)
		}
		if column, found := FIELD_NAMES[keyvalue[0]]; !found {
			return nil, fmt.Errorf("unknown field name %s", keyvalue[0])
		} else if _, found := conditions[column]; found {
			conditions[column] = append(conditions[column], keyvalue[1])
		} else {
			conditions[column] = []string{keyvalue[1]}
		}
	}

	return conditions, nil
}

func parseDuration(duration string) (time.Time, error) {
	t := time.Now()
	if duration != "now" {
		re := regexp.MustCompile(`^(\d+)([smhdM])$`)
		matches := re.FindStringSubmatch(duration)
		if len(matches) != 3 {
			return t, fmt.Errorf("invalid duration %s", duration)
		}
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			return t, fmt.Errorf("invalid duration %s", duration)
		}

		switch matches[2] {
		case "s":
			t = t.Add(-time.Duration(number) * time.Second)
		case "m":
			t = t.Add(-time.Duration(number) * time.Minute)
		case "h":
			t = t.Add(-time.Duration(number) * time.Hour)
		case "d":
			t = t.Add(-time.Duration(number) * time.Hour * 24)
		case "M":
			t = t.Add(-time.Duration(number) * time.Hour * 24 * 30)
		}
	}
	return t, nil
}
