package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
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

// Use a var to get current time, allowing for tests to override it
var NowTimeFun = time.Now

// defaulting to the default Debian location (and presumably other linuxes)
// overridable with NGTOP_LOGS_PATH env var
const DEFAULT_PATH_PATTERN = "/var/log/nginx/access.log*"
const DEFAULT_DB_PATH = "./ngtop.db"

// TODO replace with 'combined' once alias support is added
const DEFAULT_LOG_FORMAT = `$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"`

func main() {
	// Optionally enable internal logger
	if os.Getenv("NGTOP_LOG") == "" {
		log.Default().SetOutput(io.Discard)
	}

	dbPath := DEFAULT_DB_PATH
	if envPath := os.Getenv("NGTOP_DB"); envPath != "" {
		dbPath = envPath
	}

	logPathPattern := DEFAULT_PATH_PATTERN
	if envLogsPath := os.Getenv("NGTOP_LOGS_PATH"); envLogsPath != "" {
		logPathPattern = envLogsPath
	}

	logFormat := DEFAULT_LOG_FORMAT
	if envLogFormat := os.Getenv("NGTOP_LOG_FORMAT"); envLogFormat != "" {
		logFormat = envLogFormat
	}

	parser := NewParser(logFormat)

	ctx, spec := querySpecFromCLI()
	dbs, err := InitDB(dbPath, parser.Fields)
	ctx.FatalIfErrorf(err)
	defer dbs.Close()

	err = loadLogs(parser, logPathPattern, dbs)
	ctx.FatalIfErrorf(err)

	columnNames, rowValues, err := dbs.QueryTop(spec)
	ctx.FatalIfErrorf(err)
	printTopTable(columnNames, rowValues)
}

// Parse the command line arguments into a top requests query specification
func querySpecFromCLI() (*kong.Context, *RequestCountSpec) {
	// Parse query spec first, i.e. don't bother with db updates if the command is invalid
	fieldNames := make([]string, 0, len(CLI_NAME_TO_FIELD))
	for k := range CLI_NAME_TO_FIELD {
		fieldNames = append(fieldNames, k)
	}

	cli := CommandArgs{}
	ctx := kong.Parse(
		&cli,
		kong.Description("ngtop prints request counts from nginx access.logs based on a command-line query"),
		kong.UsageOnError(),
		kong.Vars{
			"version": "jorge v0.2.0",
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
		columns[i] = CLI_NAME_TO_FIELD[field].ColumnName
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

// Parse the -w conditions like "ua=Firefox" and "url=/blog%" into a mapping that can be used to query the database.
// field alias are translated to their canonical column name
// multiple values of the same field are preserved to be used as OR values
// different fields will be treated as AND conditions on the query
// != pairs are treated as 'different than'
func resolveWhereConditions(clauses []string) (map[string][]string, error) {
	conditions := make(map[string][]string)

	for _, clause := range clauses {
		// for non equal conditions, leave a trailing '!' in the value
		clause = strings.Replace(clause, "!=", "=!", 1)

		keyvalue := strings.Split(clause, "=")
		if len(keyvalue) != 2 {
			return nil, fmt.Errorf("invalid where expression %s", clause)
		}

		if field, found := CLI_NAME_TO_FIELD[keyvalue[0]]; found {
			conditions[field.ColumnName] = append(conditions[field.ColumnName], keyvalue[1])
		} else {
			return nil, fmt.Errorf("unknown field name %s", keyvalue[0])
		}
	}

	return conditions, nil
}

// parse duration expressions as 1d or 10s into a date by subtracting them from the Now() time.
func parseDuration(duration string) (time.Time, error) {
	t := NowTimeFun().UTC()
	if duration != "now" {
		re := regexp.MustCompile(`^(\d+)([smhdwM])$`)
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

// Parse the most recent nginx access.logs and insert the ones not previously seen into the DB.
func loadLogs(parser *LogParser, logPathPattern string, dbs *dbSession) error {
	logFiles, err := filepath.Glob(logPathPattern)
	if err != nil {
		return err
	}

	// Get the last log time to know when to stop parsing, and prepare a transaction to insert newer entries
	lastSeenTime, err := dbs.PrepareForUpdate()
	if err != nil {
		return err
	}

	err = parser.Parse(logFiles, lastSeenTime, func(logLineFields map[string]string) error {
		queryValues := make([]interface{}, len(dbs.columns))
		for i, field := range dbs.columns {
			queryValues[i] = logLineFields[field]
		}
		return dbs.AddLogEntry(queryValues...)
	})

	// Rollback or commit before returning, depending on the error value
	return dbs.FinishUpdate(err)
}

// Print the query results as a table
func printTopTable(columnNames []string, rowValues [][]string) {
	tab := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintf(tab, "%s\n", strings.ToUpper(strings.Join(columnNames, "\t")))
	for _, row := range rowValues {
		row[len(row)-1] = prettyPrintCount(row[len(row)-1])
		fmt.Fprintf(tab, "%s\n", strings.Join(row, "\t"))
	}
	tab.Flush()
}

func prettyPrintCount(countStr string) string {
	// FIXME some unnecessary work, first db stringifies, then this parses to int, then formats again.
	// this suggests the query implementation and/or APIs could be made smarter
	n, _ := strconv.Atoi(countStr)
	if n == 0 {
		return "0"
	}

	// HAZMAT: authored by chatgpt
	// Define suffixes and corresponding values
	suffixes := []string{"", "K", "M", "B", "T"}
	base := 1000.0
	absValue := math.Abs(float64(n))
	magnitude := int(math.Floor(math.Log(absValue) / math.Log(base)))
	value := absValue / math.Pow(base, float64(magnitude))

	if magnitude == 0 {
		// No suffix, present as an integer
		return fmt.Sprintf("%d", n)
	} else {
		// Use the suffix and present as a float with 1 decimal place
		return fmt.Sprintf("%.1f%s", value, suffixes[magnitude])
	}
}
