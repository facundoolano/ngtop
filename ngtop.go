package main

import (
	"bufio"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

const LOG_COMBINED_PATTERN = `(?P<remote_addr>\S+) - (?P<remote_user>\S+) \[(?P<time_local>.*?)\] "(?P<request>[^"]*)" (?P<status>\d{3}) (?P<body_bytes_sent>\d+) "(?P<http_referer>[^"]*)" "(?P<http_user_agent>[^"]*)"`

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

	sqlStmt := `
		CREATE TABLE IF NOT EXISTS access_logs (
			id 				INTEGER NOT NULL PRIMARY KEY,

			ip				TEXT,
			time 			TIMESTAMP NOT NULL,
			request_raw		TEXT NOT NULL,
			status			INTEGER,
			bytes_sent		INTEGER,
			referrer 		TEXT,
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

		logPattern := regexp.MustCompile(LOG_COMBINED_PATTERN)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parsed := parseLogLine(logPattern, line)
			if parsed == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			// TODO insert into table

			log.Print(parsed)
		}
		checkError(scanner.Err())

	}

	// TODO accept more input files
	// load all rows from input files

}

func parseLogLine(pattern *regexp.Regexp, logLine string) map[string]string {
	match := pattern.FindStringSubmatch(logLine)
	if match == nil {
		return nil
	}
	result := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	// TODO rename fields
	// TODO add non raw
	return result
}

func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}
