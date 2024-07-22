package main

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Optionally enable internal logger
	if os.Getenv("NGTOP_LOG") == "" {
		log.Default().SetOutput(io.Discard)
	}

	dbPath := "./ngtop.db"
	if envPath := os.Getenv("NGTOP_DB"); envPath != "" {
		dbPath = envPath
	}
	db := initDB(dbPath)
	defer db.Close()

	// defaulting to the default Debian location (and presumably other linuxes)
	// could make sense to try detecting the OS and applying a sensible default accordingly
	accessLogsPath := "/var/log/ngninx/access.log*"
	if envLogsPath := os.Getenv("NGTOP_LOGS_PATH"); envLogsPath != "" {
		accessLogsPath = envLogsPath
	}
	filePaths, err := filepath.Glob(accessLogsPath)
	checkError(err)
	loadLogs(db, filePaths...)

	spec := querySpecFromCLI()
	runTopQuery(db, spec)
}

func initDB(dbPath string) *sql.DB {
	db, err := sql.Open("sqlite3", dbPath)
	checkError(err)
	_, err = db.Exec("PRAGMA journal_mode=memory;")
	checkError(err)

	sqlStmt := `
		CREATE TABLE IF NOT EXISTS access_logs (
			id 				INTEGER NOT NULL PRIMARY KEY,

			ip				TEXT,
			time 			TIMESTAMP NOT NULL,
			request_raw		TEXT NOT NULL,
			status			INTEGER,
			bytes_sent		INTEGER,
			referer 		TEXT,
			user_agent_raw 	TEXT,

			method			TEXT,
			path			TEXT,
			user_agent	 	TEXT,

			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(sqlStmt)
	checkError(err)

	// TODO add indexes according to expected queries

	return db
}

func runTopQuery(db *sql.DB, query RequestCountSpec) {
	rows, err := query.Exec(db)
	checkError(err)
	defer rows.Close()

	tab := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)

	// FIXME this code is a mess
	columns, err := rows.Columns()
	checkError(err)
	values := make([]interface{}, len(columns))
	strValues := make([]string, len(columns))
	for i := range len(columns) {
		values[i] = new(sql.RawBytes)
	}

	fmt.Fprintf(tab, "%s\n", strings.ToUpper(strings.Join(columns, "\t")))
	for rows.Next() {
		checkError(rows.Scan(values...))
		for i, value := range values {
			strValues[i] = fmt.Sprintf("%s", *value.(*sql.RawBytes))
		}
		fmt.Fprintf(tab, "%s\n", strings.Join(strValues, "\t"))
	}
	tab.Flush()
	checkError(rows.Err())
}

// TODO add support to nginx config syntax, eg "$remote_addr - $remote_user [$time_local] ..."
// and add code to translate it to these regexes
const LOG_COMBINED_PATTERN = `(?P<ip>\S+) - (?P<remote_user>\S+) \[(?P<time>.*?)\] "(?P<request_raw>[^"]*)" (?P<status>\d{3}) (?P<bytes_sent>\d+) "(?P<referer>[^"]*)" "(?P<user_agent_raw>[^"]*)"`

var logPattern = regexp.MustCompile(LOG_COMBINED_PATTERN)
var insertFields = []string{"ip", "time", "request_raw", "status", "bytes_sent", "referer", "user_agent_raw", "method", "path", "user_agent"}
var insertValuePlaceholder = strings.TrimSuffix(strings.Repeat("?,", len(insertFields)), ",")

func loadLogs(db *sql.DB, logFiles ...string) {

	// we want to avoid processed files that were already processed in the past.  but we still want to add new log entries
	// from the most recent files, which may have been extended since we last saw them.
	// Since there is no "uniqueness" in logs (even the same ip can make the same request at the same second ---I checked),
	// I remove the entries with the highest timestamp, and load everything up until including that timestamp but not older.
	// The assumption is that any processing was completely finished, not interrupted.
	// TODO: we may want to arrange the transactions to guarantee that assumption
	var lastSeenTimeStr string
	var lastSeenTime time.Time
	isDiffLoad := false
	if err := db.QueryRow("SELECT max(time) FROM access_logs").Scan(&lastSeenTimeStr); err == nil {
		_, err := db.Exec("DELETE FROM access_logs WHERE time = ?", lastSeenTimeStr)
		checkError(err)

		isDiffLoad = true
		lastSeenTime, err = timeFromDBFormat(lastSeenTimeStr)
		checkError(err)
	}

	// have a single transaction for all the insert
	tx, err := db.Begin()
	checkError(err)
	defer func() {
		checkError(tx.Commit())
	}()
	insertStmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO access_logs(%s) values(%s);", strings.Join(insertFields, ","), insertValuePlaceholder))
	checkError(err)

	for _, path := range logFiles {

		log.Printf("parsing %s", path)
		file, err := os.Open(path)
		checkError(err)
		defer file.Close()

		// if it's gzipped, wrap in a decompressing reader
		var reader io.Reader = file
		if filepath.Ext(path) == ".gz" {
			gz, err := gzip.NewReader(file)
			checkError(err)
			reader = gz
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			values := parseLogLine(line)
			if values == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			if isDiffLoad && values["time"].(time.Time).Compare(lastSeenTime) < 0 {
				// already caught up, no need to continue processing
				return
			}

			queryValues := make([]interface{}, len(insertFields))
			for i, field := range insertFields {
				queryValues[i] = values[field]
			}
			_, err := insertStmt.Exec(queryValues...)
			checkError(err)
		}
		checkError(scanner.Err())
	}
}

// TODO this should be reader interface wrapping another reader
func parseLogLine(logLine string) map[string]interface{} {
	match := logPattern.FindStringSubmatch(logLine)
	if match == nil {
		return nil
	}
	result := make(map[string]interface{})
	for i, name := range logPattern.SubexpNames() {
		if i != 0 && name != "" && match[i] != "-" {
			result[name] = match[i]
		}
	}

	// assuming all the fields were found otherwise there would be no match above

	// parse log time to time.Time
	time, err := timeFromLogFormat(result["time"].(string))
	checkError(err)
	result["time"] = time

	// bytes as integer
	bytes_sent, _ := strconv.Atoi(result["bytes_sent"].(string))
	result["bytes_sent"] = bytes_sent

	// status as integer
	status, _ := strconv.Atoi(result["status"].(string))
	result["status"] = status

	// FIXME normalize user agent
	result["user_agent"] = result["user_agent_raw"]

	request_parts := strings.Split(result["request_raw"].(string), " ")
	if len(request_parts) == 3 {
		// if the request line is weird, don't try to extract its fields
		result["method"] = request_parts[0]
		raw_path := request_parts[1]
		if url, err := url.Parse(raw_path); err == nil {
			result["path"] = url.Path
		} else {
			result["path"] = raw_path
		}
	}

	return result
}

func timeFromLogFormat(timestamp string) (time.Time, error) {
	clfLayout := "02/Jan/2006:15:04:05 -0700"
	return time.Parse(clfLayout, timestamp)
}

// FIXME refactor into db file
func timeFromDBFormat(timestamp string) (time.Time, error) {
	sqliteLayout := "2006-01-02 15:04:05-07:00"
	return time.Parse(sqliteLayout, timestamp)
}

// TODO think if this is reasonable enough for all cases
func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}
