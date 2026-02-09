package drupalupdate

import (
	"regexp"
	"strconv"
	"strings"
)

// Version holds parsed segments of a version string (e.g. from drupal.org or Packagist).
type Version struct {
	Prefix    string // e.g. "8.x", "12.x", ... or ""
	Stability string // "", "RC", "beta", "alpha"

	// Regular version segments, -1 indicates missing segment.
	Major, Minor, Patch int
}

var (
	versionRegex      = regexp.MustCompile(`^((\d+\.x)-)?((\d+)(\.\d+){0,2})(-([aA-zZ]+).*)?$`) // regular expression used to parse a version
	stabilityPrefixes = []string{"RC", "beta", "alpha"}                                         // version strings
)

// ParseVersion parses a raw version string into a Version.
// Handles optional "8.x-" prefix and stability suffixes (-rc*, -beta*, -alpha*).
// Numeric parts are parsed segment by segment; non-numeric segments are treated as 0.
func ParseVersion(s string) (v Version) {
	// default all the versions to -1
	v.Major, v.Minor, v.Patch = -1, -1, -1

	matches := versionRegex.FindAllStringSubmatch(s, 1)
	if len(matches) == 0 {
		return v
	}

	match := matches[0]
	if len(match) != 8 {
		panic("never reached: match must have length 8")
	}

	v.Prefix = match[2]

	digits := strings.SplitN(match[3], ".", 3)
	if len(digits) > 0 {
		v.Major = parseLeadingInt(digits[0])
	}
	if len(digits) > 1 {
		v.Minor = parseLeadingInt(digits[1])
	}
	if len(digits) > 2 {
		v.Patch = parseLeadingInt(digits[2])
	}

	v.Stability = parseStability(match[7])

	return v
}

func parseStability(s string) string {
	for _, prefix := range stabilityPrefixes {
		if !strings.EqualFold(s, prefix) {
			continue
		}
		return prefix
	}
	return ""
}

// VersionPin returns the composer version constraint for this version.
// Drops patch, prepends "^", and appends @RC/@beta/@alpha when applicable.
//
//	Major=5, Minor=0, Patch=3 → "^5.0"
//	Stability="RC" → "^1.0@RC"
func (v Version) VersionPin() string {
	pin := "^"
	if v.Minor >= 0 {
		pin += strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor)
	} else {
		pin += strconv.Itoa(v.Major)
	}
	if v.Stability != "" {
		pin += "@" + v.Stability
	}
	return pin
}

// Compare returns a comparison result: >0 if v > other, <0 if v < other, 0 if equal.
// Used for sorting (e.g. descending = other.Compare(v)).
// Missing segment (-1) is treated as 0 for comparison.
func (v Version) Compare(other Version) int {
	ma, mb := seg(v.Major), seg(other.Major)
	if ma != mb {
		return ma - mb
	}
	miA, miB := seg(v.Minor), seg(other.Minor)
	if miA != miB {
		return miA - miB
	}
	return seg(v.Patch) - seg(other.Patch)
}

func seg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// parseLeadingInt parses the leading integer from a string like "3" or "3-rc1".
// Returns the fallback value if no integer is found.
func parseLeadingInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
