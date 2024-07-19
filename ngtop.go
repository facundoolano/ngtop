package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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

	// FIXME use a standard location by default
	// FIXME allow override via cli arg or config
	dbInit("./ngtop.db", cli.Paths...)

	// err := ctx.Run()
	// ctx.FatalIfErrorf(err)
}

func dbInit(dbPath string, logFiles ...string) {
	db, err := sql.Open("sqlite3", dbPath)
	checkError(err)
	defer db.Close()

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

	// TODO figure out best approach to skip already loaded
	// without missing logs from partial/errored/missed files

	logPattern := regexp.MustCompile(LOG_COMBINED_PATTERN)
	fields := []string{"ip", "time", "request_raw", "status", "bytes_sent", "referer", "user_agent_raw", "method", "path", "user_agent"}
	valuePlaceholder := strings.TrimSuffix(strings.Repeat("?,", len(fields)), ",")

	for _, path := range logFiles {
		// FIXME add zipped file support, don't rely on extension
		if filepath.Ext(path) == ".gz" {
			log.Printf("skipping zipped file %s", path)
			continue
		}

		log.Printf("parsing %s", path)
		file, err := os.Open(path)
		checkError(err)
		defer file.Close()

		scanner := bufio.NewScanner(file)
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
	// FIXME normalize path
	// FIXME normalize user agent

	bytes_sent, _ := strconv.Atoi(result["bytes_sent"].(string))
	result["bytes_sent"] = bytes_sent

	status, _ := strconv.Atoi(result["status"].(string))
	result["status"] = status
	result["user_agent"] = result["user_agent_raw"]

	request_parts := strings.Split(result["request_raw"].(string), " ")
	if len(request_parts) < 3 {
		// if the request line is weird, don't try to extract its fields
		result["method"] = request_parts[0]
		result["path"] = request_parts[1]
	}

	return result
}

func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}
