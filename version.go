package drupalupdate

import (
	"strconv"
	"strings"
)

// Version holds parsed segments of a version string (e.g. from drupal.org or Packagist).
type Version struct {
	Prefix    string // e.g. "8.x" or ""
	Stability string // "", "RC", "beta", "alpha"

	// Regular version segments.
	// <0 indicates missing semgent.
	Major, Minor, Patch int
}

// ParseVersion parses a raw version string into a Version.
// Handles optional "8.x-" prefix and stability suffixes (-rc*, -beta*, -alpha*).
// Numeric parts are parsed segment by segment; non-numeric segments are treated as 0.
func ParseVersion(s string) (v Version) {
	rest := s

	// Prefix
	if strings.HasPrefix(rest, "8.x-") {
		v.Prefix = "8.x"
		rest = strings.TrimPrefix(rest, "8.x-")
	}

	// Stability suffix (strip and set)
	lower := strings.ToLower(rest)
	if idx := strings.Index(lower, "-rc"); idx != -1 {
		rest = rest[:idx]
		v.Stability = "RC"
	} else if idx := strings.Index(lower, "-beta"); idx != -1 {
		rest = rest[:idx]
		v.Stability = "beta"
	} else if idx := strings.Index(lower, "-alpha"); idx != -1 {
		rest = rest[:idx]
		v.Stability = "alpha"
	}

	// Major, minor, patch (-1 means not present)
	v.Major = -1
	v.Minor = -1
	v.Patch = -1
	parts := strings.SplitN(rest, ".", 4)
	if len(parts) >= 1 {
		v.Major = parseLeadingInt(parts[0])
	}
	if len(parts) >= 2 {
		v.Minor = parseLeadingInt(parts[1])
	}
	if len(parts) >= 3 {
		v.Patch = parseLeadingInt(parts[2])
	}
	return v
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
