package main

import (
	"database/sql"
	"fmt"
	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

// TODO add arg to parse log files
var cli struct {
}

func main() {
	ctx := kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.HelpOptions{FlagsLast: true},
		kong.Vars{"version": "ngtop v0.1.0"},
	)

	fmt.Println("Hello, world.")

	// FIXME use a standard location by default
	// FIXME allow override via cli arg or config
	dbInit("./ngtop.db")

	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

func dbInit(dbPath string, logFiles ...string) {
	db, err := sql.Open("sqlite3", dbPath)
	checkErr(err)
	defer db.Close()

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS access_logs (
		id 				INTEGER NOT NULL PRIMARY KEY,

		ip				TEXT,
		time 			TIMESTAMP NOT NULL,
		request_raw		TEXT,
		status			INTEGER,
		bytes_sent		INTEGER,
		referrer 		TEXT,
		user_agent_raw 	TEXT,

		method			TEXT,
		path			TEXT,
		user_agent	 	TEXT,

		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
`
	_, err = db.Exec(sqlStmt)
	checkErr(err)

	// open DB
	// if not exist create tables

	// TODO accept more input files
	// load all rows from input files

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
