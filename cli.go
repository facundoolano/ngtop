package main

import (
	"time"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

// TODO move to another file
var cli struct {
	Field string   `arg:"" optional:"" type:"columnNames" help:"TODO"`
	Since string   `short:"s" default:"1h" type:"windowDate" help:"TODO"`
	Until string   `short:"u" optional:"" type:"windowDate" help:"TODO"`
	Limit int      `short:"l" default:"5" help:"TODO"`
	Where []string `short:"w" optional:"" type:"wherePattern" help:"TODO"`
}

func buildQuerySpec() RequestCountQuery {
	kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.Vars{"version": "ngtop v0.1.0"},
	)
	// FIXME build spec based on cli
	now := time.Now()
	hourAgo := now.Add(time.Duration(-24) * time.Hour)
	return RequestCountQuery{
		ColumnGroup: []string{"path"},
		TimeSince:   hourAgo,
		TimeUntil:   now,
		Limit:       10,
	}
}
