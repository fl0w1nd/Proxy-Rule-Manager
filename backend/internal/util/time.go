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

// ParseISO parses the fixed-width ISO layout used by this backend and the
// legacy frontend, accepting both millisecond and nanosecond precision.
func ParseISO(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}
