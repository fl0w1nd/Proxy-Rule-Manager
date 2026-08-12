package util

import "time"

const isoMilliFormat = "2006-01-02T15:04:05.000Z"

// NowISO returns the current UTC time formatted like JavaScript toISOString().
func NowISO() string {
	return time.Now().UTC().Format(isoMilliFormat)
}

// FormatISO formats t using the fixed-width ISO-8601 millisecond layout.
func FormatISO(t time.Time) string {
	return t.UTC().Format(isoMilliFormat)
}
