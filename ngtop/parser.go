package ngtop

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const LOG_DATE_LAYOUT = "02/Jan/2006:15:04:05 -0700"

type LogParser struct {
	// The regular expression pattern used to extract fields from log entries.
	// Derived from a format string.
	formatRegex *regexp.Regexp
	// The list of fields that can be expected to be extracted from an entry by this parser.
	// Results from the known fields in the format variables, plus their derived fields.
	// The parser result values will be in the same order as in this slice.
	Fields []*LogField
}

// Returns a new parser instance prepared to process logs in the given format.
func NewParser(format string) *LogParser {
	parser := LogParser{
		formatRegex: formatToRegex(format),
	}
	log.Printf("log format: %s\nlog pattern: %s\n", format, parser.formatRegex)

	// pick the subset of fields deducted from the regex, plus their derived fields
	// use a map to remove duplicates
	fieldSubset := make(map[string]*LogField)
	for _, logvar := range parser.formatRegex.SubexpNames() {
		if logvar == "" {
			continue
		}
		field := LOGVAR_TO_FIELD[logvar]
		fieldSubset[field.ColumnName] = field

		for _, derived := range field.DerivedFields {
			fieldSubset[derived] = COLUMN_NAME_TO_FIELD[derived]
		}
	}

	// turn the map into a valuelist
	for _, field := range fieldSubset {
		parser.Fields = append(parser.Fields, field)
	}

	return &parser
}

// Parse the fields in the nginx access logs since the `until` time, passing them as a slice to the `processFun`,
// in the same order as they appear in `parser.Fields1`.
// Processing is interrupted when a log older than `until` is found.
// Files with '.gz' extension are gzip decompressed before processing; the rest are assumed to be plain text.
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

// Constructs a regular expression from the given format string, converting known variable names
// as expressed in nginx log format expressions (e.g. `$remote_addr`) into named capture groups
// (e.g. `(?P<remote_addr>\S+)`).
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
			for j := i + 1; j < len(format) && isVariableNameRune(chars[j]); j++ {
				varname += string(chars[j])
			}
			i += len(varname)

			// write the proper capture group to the format regex pattern
			if _, isKnownField := LOGVAR_TO_FIELD[varname]; isKnownField {
				// if the var matches a field we care to extract, use a named group
				if previousWasSpace {
					newFormat += "(?P<" + varname + ">\\S+)"
				} else {
					newFormat += "(?P<" + varname + ">.*?)"
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

func isVariableNameRune(char rune) bool {
	return (char >= 'a' && char <= 'z') || char == '_' || (char >= '0' && char <= '9')
}

// Parses the given `line` according to the given `pattern`, passing captured variables
// to parser and derived parser functions as specified by the corresponding LogField.
// Extracted fields are returned as maps with field.ColumnName as key.
func parseLogLine(pattern *regexp.Regexp, line string) (map[string]string, error) {
	match := pattern.FindStringSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("log line didn't match format:\nformat:%s\nline:%s", pattern, line)
	}

	result := make(map[string]string)
	for i, logvar := range pattern.SubexpNames() {
		field := LOGVAR_TO_FIELD[logvar]
		if logvar != "" && match[i] != "-" {
			if field.Parse != nil {
				result[field.ColumnName] = field.Parse(match[i])
			} else {
				result[field.ColumnName] = match[i]
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
