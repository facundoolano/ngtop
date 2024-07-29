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
	"time"
)

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
	parser := LogParser{
		formatRegex: formatToRegex(format),
	}

	// pick the subset of fields deducted from the regex, plus their derived fields
	// use a map to remove duplicates
	fieldSubset := make(map[string]*LogField)
	for _, name := range parser.formatRegex.SubexpNames() {
		if name == "" {
			continue
		}
		fieldSubset[name] = COLUMN_NAME_TO_FIELD[name]

		for _, derived := range COLUMN_NAME_TO_FIELD[name].DerivedFields {
			fieldSubset[derived] = COLUMN_NAME_TO_FIELD[derived]
		}
	}

	// turn the map into a valuelist
	for _, field := range fieldSubset {
		parser.Fields = append(parser.Fields, field)
	}

	return &parser
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
