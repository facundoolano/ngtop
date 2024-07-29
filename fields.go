package main

import (
	"github.com/mileusna/useragent"
	"net/url"
	"strings"
	"time"
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
