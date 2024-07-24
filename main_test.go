package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func TestFieldsParsing(t *testing.T) {

}

func TestDurationParsing(t *testing.T) {

}

func TestWhereConditionParsing(t *testing.T) {

}

func TestBasicQuery(t *testing.T) {
	logs := `xx.xx.xx.xx - - [24/Jul/2024:00:00:28 +0000] "GET /feed HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
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

	columns, rows := runCommand(t, logs, []string{})
	assertEqual(t, columns, []string{"#reqs"})
	assertEqual(t, rows[0][0], "11")

	columns, rows = runCommand(t, logs, []string{"url"})
	assertEqual(t, columns, []string{"path", "#reqs"})
	assertEqual(t, len(rows), 5)
	assertEqual(t, rows[0], []string{"/feed.xml", "3"})
	assertEqual(t, rows[1], []string{"/feed", "2"})
	assertEqual(t, rows[2][1], "1")
	assertEqual(t, rows[3][1], "1")
	assertEqual(t, rows[4][1], "1")
}

func TestDateFiltering(t *testing.T) {
	logs := `xx.xx.xx.xx - - [24/Jul/2024:00:00:28 +0000] "GET /feed HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
xx.xx.xx.xx - - [24/Jul/2024:00:00:30 +0000] "GET /feed HTTP/1.1" 301 169 "-" "feedi/0.1.0 (+https://github.com/facundoolano/feedi)"
xx.xx.xx.xx - - [24/Jul/2024:00:00:56 +0000] "GET /blog/deconstructing-the-role-playing-videogame/ HTTP/1.1" 200 14224 "-" "feedi/0.1.0 (+https://github.com/facundoolano/feedi)"
xx.xx.xx.xx - - [24/Jul/2024:00:02:17 +0000] "GET / HTTP/1.1" 200 1120 "https://olano.dev/" "SimplePie/1.8.0 (Feed Parser; http://simplepie.org; Allow like Gecko) Build/1674203855"
xx.xx.xx.xx - - [24/Jul/2024:00:04:49 +0000] "GET /blog/mi-descubrimiento-de-america HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.126 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/a-few-more-things-you-can-do-on-your-website HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/a-note-on-essential-complexity HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"
xx.xx.xx.xx - - [24/Jul/2024:00:06:41 +0000] "GET /blog/posdata-de-borges-y-bioy HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9"`

	_, rows := runCommand(t, logs, []string{})
	assertEqual(t, rows[0][0], "8")

	_, rows = runCommand(t, logs, []string{"-s", "1m"})
	assertEqual(t, rows[0][0], "3")

	_, rows = runCommand(t, logs, []string{"-u", "1m"})
	assertEqual(t, rows[0][0], "5")

	_, rows = runCommand(t, logs, []string{"-s", "4m", "-u", "1m"})
	assertEqual(t, rows[0][0], "1")

	_, rows = runCommand(t, logs, []string{"-s", "1h"})
	assertEqual(t, rows[0][0], "8")

	_, rows = runCommand(t, logs, []string{"-u", "1h"})
	assertEqual(t, rows[0][0], "0")
}

func TestLimit(t *testing.T) {
	logs := `xx.xx.xx.xx - - [24/Jul/2024:00:00:28 +0000] "GET /feed HTTP/1.1" 301 169 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
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

	_, rows := runCommand(t, logs, []string{"url"})
	assertEqual(t, len(rows), 5)
	_, rows = runCommand(t, logs, []string{"url", "-l", "3"})
	assertEqual(t, len(rows), 3)
	_, rows = runCommand(t, logs, []string{"url", "-l", "10"})
	assertEqual(t, len(rows), 8) // not that many distinct urls
}

func TestMultiField(t *testing.T) {

}

func TestWhereFilter(t *testing.T) {

}

func TestWhereMultipleValues(t *testing.T) {

}

func TestWherePattern(t *testing.T) {

}

func TestUserAgentFields(t *testing.T) {

}

func TestStatusFilter(t *testing.T) {

}

func TestCaseInsensitive(t *testing.T) {

}

func TestMultipleLogFiles(t *testing.T) {

}

//

// ------ HELPERS --------

func runCommand(t *testing.T, logs string, cliArgs []string) ([]string, [][]string) {
	// write the logs to a temp file, and point the NGTOP_LOGS_PATH env to it
	f, err := os.CreateTemp("", "access.log")
	assertEqual(t, err, nil)
	defer os.Remove(f.Name())
	_, err = f.Write([]byte(logs))
	assertEqual(t, err, nil)
	t.Setenv("NGTOP_LOGS_PATH", f.Name())

	// create a one-off db file for the test
	f, err = os.CreateTemp("", "ngtop.db")
	assertEqual(t, err, nil)
	defer os.Remove(f.Name())
	t.Setenv("NGTOP_DB", f.Name())

	// FIXME this duplicates a lot of main, perhaps we can refactor to unify the path
	cli := CommandArgs{}
	fieldNames := make([]string, 0, len(FIELD_NAMES))
	for k := range FIELD_NAMES {
		fieldNames = append(fieldNames, k)
	}
	parser, err := kong.New(&cli, kong.Vars{
		"version": "ngtop v0.1.0",
		"fields":  strings.Join(fieldNames, ","),
	})
	assertEqual(t, err, nil)
	_, err = parser.Parse(cliArgs)
	assertEqual(t, err, nil)

	spec, err := querySpecFromCLI(&cli)
	assertEqual(t, err, nil)

	dbs, err := InitDB()
	assertEqual(t, err, nil)
	defer dbs.Close()

	err = loadLogs(dbs)
	assertEqual(t, err, nil)
	columnNames, rowValues, err := dbs.QueryTop(spec)
	assertEqual(t, err, nil)
	return columnNames, rowValues
}

func TestMain(m *testing.M) {
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