package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
)

type CommandArgs struct {
	Fields []string `arg:"" optional:"" help:"TODO"`
	Since  string   `short:"s" default:"1h" help:"TODO"`
	Until  string   `short:"u" default:"now"  help:"TODO"`
	Limit  int      `short:"l" default:"5" help:"TODO"`
	Where  []string `short:"w" optional:"" help:"TODO"`
}

// FIXME consolidate field list (duplicated knowledge)
var FIELD_NAMES = map[string]string{
	"user_agent": "user_agent",
	"useragent":  "user_agent",
	"ua":         "user_agent",
	"ua_type":    "ua_type",
	"uatype":     "ua_type",
	"ua_url":     "ua_url",
	"uaurl":      "ua_url",
	"os":         "os",
	"device":     "device",
	"request":    "request_raw",
	"bytes":      "bytes_sent",
	"bytes_sent": "bytes_sent",
	"path":       "path",
	"url":        "path",
	"ip":         "ip",
	"referer":    "referer",
	"referrer":   "referer",
	"status":     "status",
	"method":     "method",
}

func main() {
	// Optionally enable internal logger
	if os.Getenv("NGTOP_LOG") == "" {
		log.Default().SetOutput(io.Discard)
	}

	// Parse query spec first, i.e. don't bother with db updates if the command is invalid
	cli := CommandArgs{}
	ctx := kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.Vars{"version": "ngtop v0.1.0"},
	)
	spec, err := querySpecFromCLI(&cli)
	ctx.FatalIfErrorf(err)

	dbs, err := InitDB()
	ctx.FatalIfErrorf(err)
	defer dbs.Close()

	err = loadLogs(dbs)
	ctx.FatalIfErrorf(err)

	columnNames, rowValues, err := dbs.QueryTop(spec)
	ctx.FatalIfErrorf(err)
	printTopTable(columnNames, rowValues)
}

func querySpecFromCLI(cli *CommandArgs) (*RequestCountSpec, error) {
	since, err := parseDuration(cli.Since)
	if err != nil {
		return nil, err
	}
	until, err := parseDuration(cli.Until)
	if err != nil {
		return nil, err
	}

	columns := make([]string, len(cli.Fields))
	for i, field := range cli.Fields {
		if value, found := FIELD_NAMES[field]; !found {
			return nil, fmt.Errorf("unknown field name %s", field)
		} else {
			columns[i] = value
		}
	}

	whereConditions, err := resolveWhereConditions(cli.Where)
	if err != nil {
		return nil, err
	}
	return &RequestCountSpec{
		GroupByMetrics: columns,
		TimeSince:      since,
		TimeUntil:      until,
		Limit:          cli.Limit,
		Where:          whereConditions,
	}, nil
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
		} else {
			conditions[column] = append(conditions[column], keyvalue[1])
		}
	}

	return conditions, nil
}

func parseDuration(duration string) (time.Time, error) {
	t := time.Now().UTC()
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

func printTopTable(columnNames []string, rowValues [][]string) {
	tab := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintf(tab, "%s\n", strings.ToUpper(strings.Join(columnNames, "\t")))
	for _, row := range rowValues {
		fmt.Fprintf(tab, "%s\n", strings.Join(row, "\t"))
	}
	tab.Flush()
}

func loadLogs(dbs *dbSession) error {
	// FIXME consolidate field list (duplicated knowledge)
	insertFields := []string{"ip", "time", "request_raw", "status", "bytes_sent", "referer", "user_agent_raw", "method", "path", "user_agent", "os", "device", "ua_url", "ua_type"}
	lastSeenTime, err := dbs.PrepareForUpdate(insertFields)
	if err != nil {
		return err
	}

	err = ProcessAccessLogs(lastSeenTime, func(fields map[string]interface{}) error {
		queryValues := make([]interface{}, len(insertFields))
		for i, field := range insertFields {
			queryValues[i] = fields[field]
		}
		return dbs.AddLogEntry(queryValues...)
	})

	// Rollback or commit before returning, depending on error
	return dbs.FinishUpdate(err)
}
