package main

import (
	"regexp"
	"strconv"
)

// Combined product version; independent of the Retail/Forever binary versions.
var productReleaseVersion = "dev"

var releaseVersionPattern = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-(retail|forever)\.([0-9]+))?$`)

// Client suffixes identify separate product channels, not interchangeable builds.
// A plain legacy release must never replace a client-specific companion.
func parseReleaseVersion(value string) ([4]int, string, bool) {
	var numbers [4]int
	parts := releaseVersionPattern.FindStringSubmatch(value)
	if parts == nil {
		return numbers, "", false
	}
	for i, part := range []string{parts[1], parts[2], parts[3], parts[5]} {
		if part == "" {
			continue
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return numbers, "", false
		}
		numbers[i] = number
	}
	return numbers, parts[4], true
}

func newer(tag, current string) bool {
	a, channel, ok := parseReleaseVersion(tag)
	b, currentChannel, currentOK := parseReleaseVersion(current)
	if !ok || !currentOK || channel != currentChannel {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
