package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"net/url"

	"github.com/mileusna/useragent"
)

// TODO
type LogField struct {
	// TODO
	LogFormatVar string
	// TODO
	CLINames []string
	// TODO
	ColumnName string
	// TODO
	ColumnSpec string
	// TODO
	Parse func(string) string
	// TODO
	DerivedFields []string
	// TODO
	ParseDerivedFields func(string) map[string]string
}

// TODO
var KNOWN_FIELDS = []LogField{
	{
		LogFormatVar: "time_local",
		ColumnName:   "time",
		ColumnSpec:   "TIMESTAMP NOT NULL",
		Parse:        parseTime,
	},
	{
		LogFormatVar:       "request",
		ColumnName:         "request_raw",
		ColumnSpec:         "TEXT",
		DerivedFields:      []string{"path", "method", "referer"},
		ParseDerivedFields: parseRequestDerivedFields,
	},
	{
		LogFormatVar:       "http_user_agent",
		ColumnName:         "user_agent_raw",
		ColumnSpec:         "TEXT",
		DerivedFields:      []string{"user_agent", "os", "device", "ua_type", "ua_url"},
		ParseDerivedFields: parseUserAgentDerivedFields,
	},
	{
		LogFormatVar: "http_referer",
		CLINames:     []string{"referer", "ref", "referrer"},
		ColumnName:   "referer",
		ColumnSpec:   "TEXT COLLATE NOCASE",
		Parse:        stripUrlSource,
	},
	{
		LogFormatVar: "remote_addr",
		CLINames:     []string{"ip"},
		ColumnName:   "ip",
		ColumnSpec:   "TEXT",
	},
	{
		LogFormatVar: "status",
		CLINames:     []string{"status"},
		ColumnName:   "status",
		ColumnSpec:   "INTEGER",
	},
	{
		CLINames:   []string{"method"},
		ColumnName: "method",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
	{
		CLINames:   []string{"path", "url"},
		ColumnName: "path",
		ColumnSpec: "TEXT",
	},
	{
		CLINames:   []string{"user_agent", "ua", "useragent"},
		ColumnName: "user_agent",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
	{
		CLINames:   []string{"os"},
		ColumnName: "os",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
	{
		CLINames:   []string{"device"},
		ColumnName: "device",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
	{
		CLINames:   []string{"ua_url", "uaurl"},
		ColumnName: "ua_url",
		ColumnSpec: "TEXT",
	},
	{
		CLINames:   []string{"ua_type", "uatype"},
		ColumnName: "ua_type",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
}

var LOGVAR_TO_FIELD = map[string]*LogField{}
var COLUMN_NAME_TO_FIELD = map[string]*LogField{}

// TODO revisit, may be better to do this at main instead
var CLI_NAME_TO_FIELD = map[string]*LogField{}

func init() {
	for _, field := range KNOWN_FIELDS {
		COLUMN_NAME_TO_FIELD[field.ColumnName] = &field
		if field.LogFormatVar != "" {
			LOGVAR_TO_FIELD[field.LogFormatVar] = &field
		}
		for _, name := range field.CLINames {
			CLI_NAME_TO_FIELD[name] = &field
		}
	}
}

const LOG_DATE_LAYOUT = "02/Jan/2006:15:04:05 -0700"

type LogParser struct {
	formatRegex *regexp.Regexp
	Fields      []*LogField
}

func NewParser(format string) *LogParser {
	regex := formatToRegex(format)

	// pick the subset of fields deducted from the regex, plus their derived fields
	// use a map to remove duplicates
	fieldSubset := make(map[string]*LogField)
	for _, name := range regex.SubexpNames() {
		if name == "" {
			continue
		}
		fieldSubset[name] = COLUMN_NAME_TO_FIELD[name]

		for _, derived := range COLUMN_NAME_TO_FIELD[name].DerivedFields {
			fieldSubset[derived] = COLUMN_NAME_TO_FIELD[derived]
		}
	}

	// turn the map into a valuelist
	fields := make([]*LogField, 0)
	for _, field := range fieldSubset {
		fields = append(fields, field)
	}

	return &LogParser{
		formatRegex: regex,
		Fields:      fields,
	}
}

// Parse the fields in the nginx access logs since the `until` time, passing them as a map into the `processFun`.
// Processing is interrupted when a log older than `until` is found.
func (parser LogParser) Parse(
	logFiles []string,
	until *time.Time,
	processFun func([]any) error,
) error {
	var untilStr string
	if until != nil {
		untilStr = until.Format(DB_DATE_LAYOUT)
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
			values, err := parseLogLine(parser.formatRegex, line)
			if err != nil {
				return err
			}
			if values == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			if untilStr != "" && values["time"] < untilStr {
				// already caught up, no need to continue processing
				return nil
			}

			valueList := make([]any, len(parser.Fields))
			for i, field := range parser.Fields {
				valueList[i] = values[field.ColumnName]
			}
			if err := processFun(valueList); err != nil {
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}

	return nil
}

// TODO
func formatToRegex(format string) *regexp.Regexp {
	chars := []rune(format)
	var newFormat string

	// previousWasSpace is used to tell, when a var is found, if it is expected to contain spaces
	// without knowing which characters are used to sorround it, eg: "$http_user_agent" [$time_local]
	previousWasSpace := true
	for i := 0; i < len(chars); i++ {
		if chars[i] != '$' {
			previousWasSpace = chars[i] == ' '
			newFormat += regexp.QuoteMeta(string(chars[i]))
		} else {
			// found a varname, process it
			varname := ""
			for j := i + 1; j < len(format) && ((chars[j] >= 'a' && chars[j] <= 'z') || chars[j] == '_'); j++ {
				varname += string(chars[j])
			}
			i += len(varname)

			// write the proper capture group to the format regex pattern
			if field, isKnownField := LOGVAR_TO_FIELD[varname]; isKnownField {
				// if the var matches a field we care to extract, use a named group
				groupname := field.ColumnName
				if previousWasSpace {
					newFormat += "(?P<" + groupname + ">\\S+)"
				} else {
					newFormat += "(?P<" + groupname + ">.*?)"
				}
			} else {
				// otherwise just add a nameless group that ensures matching
				if previousWasSpace {
					newFormat += "(?:\\S+)"
				} else {
					newFormat += "(?:.*?)"
				}
			}

		}
	}
	return regexp.MustCompile(newFormat)
}

func parseLogLine(pattern *regexp.Regexp, line string) (map[string]string, error) {
	match := pattern.FindStringSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("log line didn't match format:\nformat:%s\nline:%s", pattern, line)
	}

	result := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		field := COLUMN_NAME_TO_FIELD[name]
		if name != "" && match[i] != "-" {
			if field.Parse != nil {
				result[name] = field.Parse(match[i])
			} else {
				result[name] = match[i]
			}

			if field.ParseDerivedFields != nil {
				for key, value := range field.ParseDerivedFields(match[i]) {
					result[key] = value
				}
			}
		}
	}
	return result, nil
}

func stripUrlSource(value string) string {
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "www.")
	value = strings.TrimSuffix(value, "/")
	return value
}

// FIXME error instead of panic?
func parseTime(timestamp string) string {
	t, err := time.Parse(LOG_DATE_LAYOUT, timestamp)
	if err != nil {
		panic("can't parse log timestamp " + timestamp)
	}
	return t.Format(DB_DATE_LAYOUT)
}

func parseRequestDerivedFields(request string) map[string]string {
	result := make(map[string]string)
	request_parts := strings.Split(request, " ")
	if len(request_parts) == 3 {
		// if the request line is weird, don't try to extract its fields
		result["method"] = request_parts[0]
		raw_path := request_parts[1]
		if url, err := url.Parse(raw_path); err == nil {
			result["path"] = url.Path

			// if utm source and friends in query, use them as referer
			keys := []string{"ref", "referer", "referrer", "utm_source"}
			query := url.Query()
			for _, key := range keys {
				if query.Has(key) {
					result["referer"] = stripUrlSource(query.Get(key))
					break
				}
			}

		} else {
			result["path"] = raw_path
		}
	}

	return result
}

func parseUserAgentDerivedFields(ua string) map[string]string {
	result := make(map[string]string)
	if ua != "" {
		ua := useragent.Parse(ua)
		result["user_agent"] = ua.Name
		result["os"] = ua.OS
		result["device"] = ua.Device
		result["ua_url"] = stripUrlSource(ua.URL)
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
	return result
}
