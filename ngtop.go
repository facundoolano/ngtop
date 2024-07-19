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
	"time"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

// TODO support other formats
const LOG_COMBINED_PATTERN = `(?P<ip>\S+) - (?P<remote_user>\S+) \[(?P<time>.*?)\] "(?P<request_raw>[^"]*)" (?P<status>\d{3}) (?P<bytes_sent>\d+) "(?P<referer>[^"]*)" "(?P<user_agent_raw>[^"]*)"`

// TODO add arg to parse log files
var cli struct {
	Paths []string `arg:"" name:"path" help:"Paths to log files to ingest." type:"path"`
}

func main() {
	kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.HelpOptions{FlagsLast: true},
		kong.Vars{"version": "ngtop v0.1.0"},
	)

	// optionally disable logger
	// TODO control via an env var
	log.Default().SetOutput(io.Discard)

	// FIXME use a standard location by default
	// FIXME allow override via cli arg or config
	db := initDB("./ngtop.db")
	defer db.Close()

	loadLogs(db, cli.Paths...)
	queryTop(db)

	// err := ctx.Run()
	// ctx.FatalIfErrorf(err)
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

			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

			UNIQUE(ip, time, request_raw) ON CONFLICT REPLACE
		);
	`
	_, err = db.Exec(sqlStmt)
	checkError(err)

	// TODO add indexes according to expected queries

	return db
}

func queryTop(db *sql.DB) {
	// FIXME make this generic
	rows, err := db.Query(`
SELECT path, count(1)
FROM access_logs
WHERE time > datetime('now', '-1 month') AND status <> 301
GROUP BY 1
ORDER BY 2 DESC
LIMIT 10
`)
	checkError(err)
	defer rows.Close()

	// FIXME separate querying from presentation
	fmt.Printf("%s\t%s\n", "path", "requests")
	for rows.Next() {
		var path string
		var count int
		rows.Scan(&path, &count)
		fmt.Printf("%s\t%d\n", path, count)
	}
	checkError(rows.Err())
}

func loadLogs(db *sql.DB, logFiles ...string) {

	// TODO figure out best approach to skip already loaded
	// without missing logs from partial/errored/missed files

	logPattern := regexp.MustCompile(LOG_COMBINED_PATTERN)
	fields := []string{"ip", "time", "request_raw", "status", "bytes_sent", "referer", "user_agent_raw", "method", "path", "user_agent"}
	valuePlaceholder := strings.TrimSuffix(strings.Repeat("?,", len(fields)), ",")

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
		tx, err := db.Begin()
		checkError(err)
		insertStmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO access_logs(%s) values(%s);", strings.Join(fields, ","), valuePlaceholder))
		checkError(err)

		for scanner.Scan() {
			line := scanner.Text()
			values := parseLogLine(logPattern, line)
			if values == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			queryValues := make([]interface{}, len(fields))
			for i, field := range fields {
				queryValues[i] = values[field]
			}
			_, err := insertStmt.Exec(queryValues...)
			checkError(err)
		}
		checkError(scanner.Err())
		checkError(tx.Commit())
	}
}

func parseLogLine(pattern *regexp.Regexp, logLine string) map[string]interface{} {
	match := pattern.FindStringSubmatch(logLine)
	if match == nil {
		return nil
	}
	result := make(map[string]interface{})
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" && match[i] != "-" {
			result[name] = match[i]
		}
	}

	// assuming all the fields were found otherwise there would be no match above

	// parse log time to time.Time
	clfLayout := "02/Jan/2006:15:04:05 -0700"
	time, err := time.Parse(clfLayout, result["time"].(string))
	result["time"] = time
	checkError(err)

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

// TODO think if this is reasonable enough for all cases
func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}
