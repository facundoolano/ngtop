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
	Fields []string            `arg:"" optional:"" help:"TODO"`
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
	return RequestCountSpec{
		GroupByMetrics: cli.Fields,
		TimeSince:      since,
		TimeUntil:      until,
		Limit:          cli.Limit,
		Where:          cli.Where,
	}
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
