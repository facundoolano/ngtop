package main

import (
	"bufio"
	"compress/gzip"
	"github.com/mileusna/useragent"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// defaulting to the default Debian location (and presumably other linuxes)
// overridable with NGTOP_LOGS_PATH env var
const DEFAULT_PATH = "/var/log/ngninx/access.log*"

// TODO add support to nginx config syntax, eg "$remote_addr - $remote_user [$time_local] ..." and add code to translate it to these regexes
// FIXME consolidate field list (duplicated knowledge)
const LOG_COMBINED_PATTERN = `(?P<ip>\S+) - (?P<remote_user>\S+) \[(?P<time>.*?)\] "(?P<request_raw>[^"]*)" (?P<status>\d{3}) (?P<bytes_sent>\d+) "(?P<referer>[^"]*)" "(?P<user_agent_raw>[^"]*)"`

var logPattern = regexp.MustCompile(LOG_COMBINED_PATTERN)

func ProcessAccessLogs(until *time.Time, processFun func(map[string]interface{}) error) error {

	// could make sense to try detecting the OS and applying a sensible default accordingly
	accessLogsPath := DEFAULT_PATH
	if envLogsPath := os.Getenv("NGTOP_LOGS_PATH"); envLogsPath != "" {
		accessLogsPath = envLogsPath
	}
	logFiles, err := filepath.Glob(accessLogsPath)
	if err != nil {
		return err
	}

	for _, path := range logFiles {

		log.Printf("parsing %s", path)
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// if it's gzipped, wrap in a decompressing reader
		var reader io.Reader = file
		if filepath.Ext(path) == ".gz" {
			if reader, err = gzip.NewReader(file); err != nil {
				return err
			}
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			values, err := parseLogLine(line)
			if err != nil {
				return err
			}
			if values == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			if until != nil && values["time"].(time.Time).Compare(*until) < 0 {
				// already caught up, no need to continue processing
				return nil
			}

			if err := processFun(values); err != nil {
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}

	return nil
}

func parseLogLine(logLine string) (map[string]interface{}, error) {
	match := logPattern.FindStringSubmatch(logLine)
	if match == nil {
		return nil, nil
	}
	result := make(map[string]interface{})
	for i, name := range logPattern.SubexpNames() {
		if i != 0 && name != "" && match[i] != "-" {
			result[name] = match[i]
		}
	}

	// assuming all the fields were found otherwise there would be no match above

	// parse log time to time.Time
	time, err := timeFromLogFormat(result["time"].(string))
	if err != nil {
		return nil, err
	}
	result["time"] = time

	// bytes as integer
	bytes_sent, _ := strconv.Atoi(result["bytes_sent"].(string))
	result["bytes_sent"] = bytes_sent

	// status as integer
	status, _ := strconv.Atoi(result["status"].(string))
	result["status"] = status

	if ua, found := result["user_agent_raw"]; found {
		ua := useragent.Parse(ua.(string))
		result["user_agent"] = ua.Name
		result["os"] = ua.OS
		result["device"] = ua.Device
		result["ua_url"] = strings.TrimPrefix(ua.URL, "https://")
		if ua.Bot {
			result["ua_type"] = "bot"
		} else if ua.Tablet {
			result["ua_type"] = "tablet"
		} else if ua.Mobile {
			result["ua_type"] = "mobile"
		} else if ua.Desktop {
			result["ua_type"] = "desktop"
		}
	}

	if referer, found := result["referer"]; found {
		result["referer"] = strings.TrimPrefix(referer.(string), "https://")
	}

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

	return result, nil
}

func timeFromLogFormat(timestamp string) (time.Time, error) {
	clfLayout := "02/Jan/2006:15:04:05 -0700"
	return time.Parse(clfLayout, timestamp)
}
