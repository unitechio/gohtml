// Package common contains common properties used by the subpackages.
package common

import "time"

const (
	versionString = "0.14.0"
	formatLayout  = "2 January 2006 at 15:04"
)

var (
	ReleasedAt = time.Date(2025, time.March, 9, 12, 20, 0, 0, time.UTC)
	Version    = versionString
)

// UtcTimeFormat returns a formatted string describing a UTC timestamp.
func UtcTimeFormat(t time.Time) string {
	return t.Format(formatLayout) + " UTC"
}
