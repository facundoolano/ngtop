package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"net/url"

	"github.com/mileusna/useragent"
)

type LogField struct {
	LogFormatVar       string
	CLINames           []string
	ColumnName         string
	ColumnSpec         string
	Parse              func(string) string
	ParseDerivedFields func(string) map[string]string
}

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
		ParseDerivedFields: parseRequestDerivedFields,
	},
	{
		LogFormatVar:       "http_user_agent",
		ColumnName:         "user_agent_raw",
		ColumnSpec:         "TEXT",
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

// TODO
func FormatToRegex(format string) *regexp.Regexp {
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
		if i != 0 && name != "" && match[i] != "-" {
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
	logLayout := "02/Jan/2006:15:04:05 -0700"
	dbLayout := "2006-01-02 15:04:05-07:00"
	t, err := time.Parse(logLayout, timestamp)
	if err != nil {
		panic("can't parse log timestamp " + timestamp)
	}
	return t.Format(dbLayout)
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
