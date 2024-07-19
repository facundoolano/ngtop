package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

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
		log.Printf("parsing %s", path)
		srcFile, err := os.Open(path)
		checkError(err)
		defer srcFile.Close()
	}

	// TODO accept more input files
	// load all rows from input files

}

func checkError(err error) {
	if err != nil {
		log.Panic(err)
	}
}
