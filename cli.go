package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

// FIXME move this to main
var cli struct {
	Fields []string            `arg:"" optional:"" enum:"ip,url,path,request,bytes,ua,user_agent,useragent,method,status,referer" help:"TODO"`
	Since  string              `short:"s" default:"1h" help:"TODO"`
	Until  string              `short:"u" default:"now"  help:"TODO"`
	Limit  int                 `short:"l" default:"5" help:"TODO"`
	Where  map[string][]string `short:"w" optional:"" help:"TODO"`
}

func buildQuerySpec() RequestCountSpec {
	kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.Vars{"version": "ngtop v0.1.0"},
	)

	since, err := parseDuration(cli.Since)
	checkError(err)
	until, err := parseDuration(cli.Until)
	checkError(err)

	columns := make([]string, len(cli.Fields))
	for i, field := range cli.Fields {
		columns[i] = resolveColumn(field)
	}

	return RequestCountSpec{
		GroupByMetrics: columns,
		TimeSince:      since,
		TimeUntil:      until,
		Limit:          cli.Limit,
		Where:          cli.Where,
	}
}

func resolveColumn(column string) string {
	switch column {
	// FIXME should prefer parsed user_agent when that's available
	case "user_agent":
	case "useragent":
	case "ua":
		return "user_agent_raw"
	case "request":
		return "request_raw"
	case "bytes":
		return "bytes_sent"
	case "url":
		return "path"
	}
	return column
}

func parseDuration(duration string) (time.Time, error) {
	t := time.Now()
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
