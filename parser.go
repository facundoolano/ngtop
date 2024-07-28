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
		LogFormatVar: "$time_local",
		ColumnName:   "time",
		ColumnSpec:   "TIMESTAMP NOT NULL",
		Parse:        parseTime,
	},
	{
		LogFormatVar:       "$request",
		ColumnName:         "request_raw",
		ColumnSpec:         "TEXT",
		ParseDerivedFields: parseRequestDerivedFields,
	},
	{
		LogFormatVar:       "$http_user_agent",
		ColumnName:         "user_agent_raw",
		ColumnSpec:         "TEXT",
		ParseDerivedFields: parseUserAgentDerivedFields,
	},
	{
		LogFormatVar: "$http_referer",
		CLINames:     []string{"referer", "ref", "referrer"},
		ColumnName:   "referer",
		ColumnSpec:   "TEXT COLLATE NOCASE",
		Parse:        stripUrlSource,
	},

	// TODO ip
	// TODO status
	// TODO method
	// TODO path
	// TODO user_agent
	// TODO os
	// TODO device
	// TODO ua_url
	// TODO ua_type
}

// FIXME populate in init
var LOGVAR_TO_NAME = map[string]string{}
var NAME_TO_FIELD = map[string]LogField{}

func FormatToRegex(format string) *regexp.Regexp {
	return regexp.MustCompile(formatRegexString(format))
}

// FIXME remove?
func formatRegexString(format string) string {
	chars := []rune(format)
	var newFormat string

	// comesFromSpace is used to tell, when a var is found, if it is expected to contain spaces
	// without knowing which characters are used to sorround it, eg: "$http_user_agent" [$time_local]
	// FIXME maybe better to preserve previous char
	comesFromSpace := true
	for i := 0; i < len(chars); i++ {
		if chars[i] != '$' {
			comesFromSpace = chars[i] == ' '
			newFormat += regexp.QuoteMeta(string(chars[i]))
		} else {
			// found a varname, process it
			i++
			varname := ""
			for ; i < len(format) && ((chars[i] >= 'a' && chars[i] <= 'z') || chars[i] == '_'); i++ {
				varname += string(chars[i])
			}
			if groupname, knownVar := LOGVAR_TO_NAME[varname]; knownVar {
				if comesFromSpace {
					newFormat += "(?P<" + groupname + ">\\S+)"
				} else {
					newFormat += "(?P<" + groupname + ">.*?)"
				}
			} else {
				if comesFromSpace {
					newFormat += "(?:\\S+)"
				} else {
					newFormat += "(?:.*?)"
				}
			}

		}
	}
	return newFormat
}

func parseLogLine(pattern *regexp.Regexp, line string) (map[string]string, error) {
	match := pattern.FindStringSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("log line didn't match format:\nformat:%s\nline:%s", pattern, line)
	}

	result := make(map[string]string)
	for i, name := range pattern.SubexpNames() {
		if i != 0 && name != "" && match[i] != "-" {
			result[name] = match[i]
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
