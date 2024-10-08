package ngtop

import (
	"github.com/mileusna/useragent"
	"net/url"
	"strings"
	"time"
)

// This struct provides a declarative specification of a field
// that can be found in an access log or derived from one.
// Its attributes configure how the field should be parsed in logs,
// stored in the db and referred to in the cli.
type LogField struct {

	// The variable name for this field in the log format (without the leading $), e.g. `remote_addr`.
	// Empty for derived fields.
	LogFormatVar string
	// The list of aliases this field can be referred by in the CLI e.g. `{"user_agen", "ua"}`
	CLINames []string
	// The name used to in the DB column for the field. This is considered its canonical name in this program.
	ColumnName string
	// The SQL column specification, e.g. `"TEXT COLLATE NOCASE"` for case insensitive strings
	ColumnSpec string
	// An optional parse function to transform the value extracted from the log field.
	Parse func(string) string
	// A list of fields that can be derived from the original log value.
	// E.g., `{"path", "method", "referer"}` for the `request` field.
	DerivedFields []string
	// An optional function that extracts derived fields from this one and returns them as a map.
	// The key of the map should be the ColumnName of the derived field.
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
		LogFormatVar: "time_iso8601",
		ColumnName:   "time",
		ColumnSpec:   "TIMESTAMP NOT NULL",
		Parse:        parseIsoTime,
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
		LogFormatVar: "remote_user",
		CLINames:     []string{"user", "username", "remote_user", "remoteuser"},
		ColumnName:   "user",
		ColumnSpec:   "TEXT",
	},
	{
		LogFormatVar: "status",
		CLINames:     []string{"status"},
		ColumnName:   "status",
		ColumnSpec:   "INTEGER",
	},
	{
		LogFormatVar: "uri",
		CLINames:     []string{"path", "url", "uri"},
		ColumnName:   "path",
		ColumnSpec:   "TEXT",
	},
	{
		LogFormatVar: "host",
		CLINames:     []string{"host", "server"},
		ColumnName:   "host",
		ColumnSpec:   "TEXT",
	},
	{
		CLINames:   []string{"method"},
		ColumnName: "method",
		ColumnSpec: "TEXT COLLATE NOCASE",
	},
	{
		CLINames:   []string{"path", "url", "uri"},
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

func parseIsoTime(timestamp string) string {
	t, err := time.Parse("2006-01-02T15:04:05-07:00", timestamp)
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
