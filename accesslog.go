package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Parse the fields in the nginx access logs since the `until` time, passing them as a map into the `processFun`.
// Processing is interrupted when a log older than `until` is found.
func ProcessAccessLogs(
	logFormat string,
	logFiles []string,
	until *time.Time,
	processFun func(map[string]string) error,
) error {
	logPattern := FormatToRegex(logFormat)

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
			values, err := parseLogLine(logPattern, line)
			if err != nil {
				return err
			}
			if values == nil {
				log.Printf("couldn't parse line %s", line)
				continue
			}

			// FIXME convert until to string above and make them comparable
			// if until != nil && values["time"] < *until < 0 {
			// 	// already caught up, no need to continue processing
			// 	return nil
			// }

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

func timeFromLogFormat(timestamp string) (time.Time, error) {
	clfLayout := "02/Jan/2006:15:04:05 -0700"
	return time.Parse(clfLayout, timestamp)
}
