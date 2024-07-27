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
	Fields []string `arg:"" name:"field" optional:"" enum:"${fields}" help:"Dimensions to aggregate the results. Allowed values: ${fields} "`
	Since  string   `short:"s" default:"1h" help:"Start of the time window to filter logs. Supported units are [s]econds, [m]inutes, [h]ours, [d]ays, [w]eeks, [M]onths"`
	Until  string   `short:"u" default:"now"  help:"End of the time window to filter logs. Supported units are [s]econds, [m]inutes, [h]ours, [d]ays, [w]eeks, [M]onths"`
	Limit  int      `short:"l" default:"5" help:"Amount of results to return"`
	Where  []string `short:"w" optional:"" help:"Filter expressions. Example: -w useragent=Safari -w status=200"`
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

// Use a var to get current time, allowing for tests to override it
var NowTimeFun = time.Now

func main() {
	// Optionally enable internal logger
	if os.Getenv("NGTOP_LOG") == "" {
		log.Default().SetOutput(io.Discard)
	}

	ctx, spec := querySpecFromCLI()

	dbs, err := InitDB()
	ctx.FatalIfErrorf(err)
	defer dbs.Close()

	err = loadLogs(dbs)
	ctx.FatalIfErrorf(err)

	columnNames, rowValues, err := dbs.QueryTop(spec)
	ctx.FatalIfErrorf(err)
	printTopTable(columnNames, rowValues)
}

func querySpecFromCLI() (*kong.Context, *RequestCountSpec) {
	// Parse query spec first, i.e. don't bother with db updates if the command is invalid
	fieldNames := make([]string, 0, len(FIELD_NAMES))
	for k := range FIELD_NAMES {
		fieldNames = append(fieldNames, k)
	}

	cli := CommandArgs{}
	ctx := kong.Parse(
		&cli,
		kong.Description("ngtop prints request counts from nginx access.logs based on a command-line query"),
		kong.UsageOnError(),
		kong.Vars{
			"version": "ngtop v0.1.0",
			"fields":  strings.Join(fieldNames, ","),
		},
	)

	since, err := parseDuration(cli.Since)
	ctx.FatalIfErrorf(err)
	until, err := parseDuration(cli.Until)
	ctx.FatalIfErrorf(err)

	// translate field name aliases
	columns := make([]string, len(cli.Fields))
	for i, field := range cli.Fields {
		columns[i] = FIELD_NAMES[field]
	}

	whereConditions, err := resolveWhereConditions(cli.Where)
	ctx.FatalIfErrorf(err)

	spec := &RequestCountSpec{
		GroupByMetrics: columns,
		TimeSince:      since,
		TimeUntil:      until,
		Limit:          cli.Limit,
		Where:          whereConditions,
	}
	return ctx, spec
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
	t := NowTimeFun().UTC()
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
		case "w":
			t = t.Add(-time.Duration(number) * time.Hour * 24 * 7)
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

	// FIXME this API could be improved, why not a single call?
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
