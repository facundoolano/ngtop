package main

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestDurationParsing(t *testing.T) {
	var duration time.Time
	var err error

	now := NowTimeFun()
	assertEqual(t, now, time.Date(2024, time.July, 24, 0, 7, 0, 0, time.UTC))

	// default to now
	duration, err = parseDuration("now")
	assertEqual(t, nil, err)
	assertEqual(t, duration, now)

	// support each unit s, m, h, d, w, M
	duration, err = parseDuration("1s")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 24, 0, 6, 59, 0, time.UTC))
	duration, err = parseDuration("10s")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 24, 0, 6, 50, 0, time.UTC))

	duration, err = parseDuration("1m")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 24, 0, 6, 0, 0, time.UTC))
	duration, err = parseDuration("5m")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 24, 0, 2, 0, 0, time.UTC))

	duration, err = parseDuration("1h")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 23, 23, 7, 0, 0, time.UTC))
	duration, err = parseDuration("5h")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 23, 19, 7, 0, 0, time.UTC))

	duration, err = parseDuration("1d")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 23, 0, 7, 0, 0, time.UTC))
	duration, err = parseDuration("5d")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 19, 0, 7, 0, 0, time.UTC))

	duration, err = parseDuration("1w")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 17, 0, 7, 0, 0, time.UTC))
	duration, err = parseDuration("2w")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.July, 10, 0, 7, 0, 0, time.UTC))

	// (months are just 30 days not actual calendar months)
	duration, err = parseDuration("1M")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.June, 24, 0, 7, 0, 0, time.UTC))
	duration, err = parseDuration("2M")
	assertEqual(t, nil, err)
	assertEqual(t, duration, time.Date(2024, time.May, 25, 0, 7, 0, 0, time.UTC))

	// fail on unknown unit
	_, err = parseDuration("1x")
	assert(t, err != nil)

	// fail on bad syntax
	_, err = parseDuration("asdassd")
	assert(t, err != nil)
}

func TestWhereConditionParsing(t *testing.T) {
	var cond map[string][]string
	var err error

	cond, err = resolveWhereConditions([]string{"url=/blog", "useragent=Safari", "ua=Firefox", "referer=olano%", "os!=Windows"})
	assertEqual(t, err, nil)
	assertEqual(t, cond, map[string][]string{
		"path":       {"/blog"},
		"user_agent": {"Safari", "Firefox"},
		"referer":    {"olano%"},
		"os":         {"!Windows"},
	})

	// include error on unknown field
	_, err = resolveWhereConditions([]string{"pepe=/blog"})
	assert(t, err != nil)

	// include bad syntax
	_, err = resolveWhereConditions([]string{"url"})
	assert(t, err != nil)
}

const SAMPLE_LOGS = `xx.xx.xx.xx - - [24/Jul/2024:00:00:28 +0000] "GET /feed HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
xx.xx.xx.xx - - [24/Jul/2024:00:00:30 +0000] "GET /feed HTTP/1.1" 301 169 "-" "feedi/0.1.0 (+https://github.com/facundoolano/feedi)"
xx.xx.xx.xx - - [24/Jul/2024:00:00:56 +0000] "GET /blog/deconstructing-the-role-playing-videogame/ HTTP/1.1" 200 14224 "-" "feedi/0.1.0 (+https://github.com/facundoolano/feedi)"
xx.xx.xx.xx - - [24/Jul/2024:00:01:18 +0000] "GET /feed.xml HTTP/1.1" 200 9641 "https://olano.dev/feed.xml" "FreshRSS/1.24.0 (Linux; https://freshrss.org)"
xx.xx.xx.xx - - [24/Jul/2024:00:01:20 +0000] "GET /feed.xml HTTP/1.1" 200 9641 "https://olano.dev/feed.xml" "FreshRSS/1.24.0 (Linux; https://freshrss.org)"
xx.xx.xx.xx - - [24/Jul/2024:00:01:51 +0000] "GET /feed.xml HTTP/1.1" 200 9641 "https://olano.dev/feed.xml" "FreshRSS/1.24.0 (Linux; https://freshrss.org)"
xx.xx.xx.xx - - [24/Jul/2024:00:02:17 +0000] "GET / HTTP/1.1" 200 1120 "https://olano.dev/" "SimplePie/1.8.0 (Feed Parser; http://simplepie.org; Allow like Gecko) Build/1674203855"
xx.xx.xx.xx - - [24/Jul/2024:00:04:49 +0000] "GET /blog/mi-descubrimiento-de-america HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.126 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/a-few-more-things-you-can-do-on-your-website HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/a-note-on-essential-complexity HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/posdata-de-borges-y-bioy HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"`

func TestBasicQuery(t *testing.T) {
	columns, rows := runCommand(t, SAMPLE_LOGS, []string{})
	assertEqual(t, columns, []string{"#reqs"})
	assertEqual(t, rows[0][0], "11")

	columns, rows = runCommand(t, SAMPLE_LOGS, []string{"url"})
	assertEqual(t, columns, []string{"path", "#reqs"})
	assertEqual(t, len(rows), 5)
	assertEqual(t, rows[0], []string{"/feed.xml", "3"})
	assertEqual(t, rows[1], []string{"/feed", "2"})
	assertEqual(t, rows[2][1], "1")
	assertEqual(t, rows[3][1], "1")
	assertEqual(t, rows[4][1], "1")
}

func TestDateFiltering(t *testing.T) {
	_, rows := runCommand(t, SAMPLE_LOGS, []string{})
	assertEqual(t, rows[0][0], "11")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"-s", "1m"})
	assertEqual(t, rows[0][0], "3")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"-u", "1m"})
	assertEqual(t, rows[0][0], "8")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"-s", "4m", "-u", "1m"})
	assertEqual(t, rows[0][0], "1")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"-s", "1h"})
	assertEqual(t, rows[0][0], "11")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"-u", "1h"})
	assertEqual(t, rows[0][0], "0")
}

func TestLimit(t *testing.T) {
	_, rows := runCommand(t, SAMPLE_LOGS, []string{"url"})
	assertEqual(t, len(rows), 5)
	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-l", "3"})
	assertEqual(t, len(rows), 3)
	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-l", "10"})
	assertEqual(t, len(rows), 8) // not that many distinct urls
}

func TestMultiField(t *testing.T) {
	columns, rows := runCommand(t, SAMPLE_LOGS, []string{"url", "method"})
	assertEqual(t, columns, []string{"path", "method", "#reqs"})
	assertEqual(t, len(rows), 5)
	assertEqual(t, rows[0], []string{"/feed.xml", "GET", "3"})
	assertEqual(t, rows[1], []string{"/feed", "GET", "2"})
	assertEqual(t, rows[2][1], "GET")
	assertEqual(t, rows[3][1], "GET")
	assertEqual(t, rows[4][1], "GET")

	columns, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "status"})
	assertEqual(t, columns, []string{"path", "status", "#reqs"})
	assertEqual(t, len(rows), 5)
	assertEqual(t, rows[0], []string{"/feed.xml", "200", "3"})
	assertEqual(t, rows[1], []string{"/feed", "301", "2"})

	columns, rows = runCommand(t, SAMPLE_LOGS, []string{"method", "status"})
	assertEqual(t, columns, []string{"method", "status", "#reqs"})
	assertEqual(t, len(rows), 2)
	assertEqual(t, rows[0], []string{"GET", "301", "6"})
	assertEqual(t, rows[1], []string{"GET", "200", "5"})

	columns, rows = runCommand(t, SAMPLE_LOGS, []string{"status", "method"})
	assertEqual(t, columns, []string{"status", "method", "#reqs"})
	assertEqual(t, len(rows), 2)
	assertEqual(t, rows[0], []string{"301", "GET", "6"})
	assertEqual(t, rows[1], []string{"200", "GET", "5"})
}

func TestWhereFilter(t *testing.T) {
	columns, rows := runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=200"})
	assertEqual(t, columns, []string{"path", "#reqs"})
	assertEqual(t, len(rows), 3)
	assertEqual(t, rows[0], []string{"/feed.xml", "3"})
	assertEqual(t, rows[1][1], "1")
	assertEqual(t, rows[2][1], "1")

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=301", "-l", "10"})
	assertEqual(t, len(rows), 5)
	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "method=GET"})
	assertEqual(t, len(rows), 5)
	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "method=get"})
	assertEqual(t, len(rows), 5)
}

func TestWhereMultipleValues(t *testing.T) {
	_, rows := runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=200", "-w", "status=301"})
	assertEqual(t, len(rows), 5)
	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=200", "-w", "status=301", "-l", "10"})
	assertEqual(t, len(rows), 8)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "ua=feedi"})
	assertEqual(t, len(rows), 2)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "ua=feedi", "-w", "status=200"})
	assertEqual(t, len(rows), 1)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "ua=feedi", "-w", "status=200", "-w", "status=301"})
	assertEqual(t, len(rows), 2)
}

func TestWherePattern(t *testing.T) {
	_, rows := runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "url=/feed%"})
	assertEqual(t, len(rows), 2)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "url=/blog/%"})
	assertEqual(t, len(rows), 5)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=3%"})
	assertEqual(t, len(rows), 5)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status=2%"})
	assertEqual(t, len(rows), 3)
}

func TestWhereNegation(t *testing.T) {
	_, rows := runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status!=200", "-l", "10"})
	assertEqual(t, len(rows), 5)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status!=301", "-l", "10"})
	assertEqual(t, len(rows), 3)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status!=2%", "-l", "10"})
	assertEqual(t, len(rows), 5)

	_, rows = runCommand(t, SAMPLE_LOGS, []string{"url", "-w", "status!=3%", "-l", "10"})
	assertEqual(t, len(rows), 3)
}

func TestMultipleLogFiles(t *testing.T) {
	// TODO
	// more than one file in a dir, honoring the glob pattern
	// include gzipped value
}

//

// ------ HELPERS --------

func runCommand(t *testing.T, logs string, cliArgs []string) ([]string, [][]string) {
	// write the logs to a temp file, and point the NGTOP_LOGS_PATH env to it
	logFile, err := os.CreateTemp("", "access.log")
	assertEqual(t, err, nil)
	defer os.Remove(logFile.Name())
	_, err = logFile.Write([]byte(logs))
	assertEqual(t, err, nil)

	// create a one-off db file for the test
	dbFile, err := os.CreateTemp("", "ngtop.db")
	assertEqual(t, err, nil)
	defer os.Remove(dbFile.Name())

	os.Args = append([]string{"ngtop"}, cliArgs...)
	_, spec := querySpecFromCLI()

	dbs, err := InitDB(dbFile.Name())
	assertEqual(t, err, nil)
	defer dbs.Close()

	err = loadLogs(DEFAULT_LOG_FORMAT, logFile.Name(), dbs)
	assertEqual(t, err, nil)
	columnNames, rowValues, err := dbs.QueryTop(spec)
	assertEqual(t, err, nil)
	return columnNames, rowValues
}

func TestMain(m *testing.M) {
	// override the time.Now function to make the since/until durations simpler to calculate
	// from the sample log dates
	NowTimeFun = func() time.Time {
		return time.Date(2024, time.July, 24, 0, 7, 0, 0, time.UTC)
	}

	m.Run()
}

func assert(t *testing.T, cond bool) {
	t.Helper()
	if !cond {
		t.Fatalf("condition is false")
	}
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%v != %v", a, b)
	}
}
