package version

import (
	"strings"
)

// FuzzyMatchVersion checks if two version strings match with fuzzy matching
// e.g., 4.32.0 matches 4.32, 4.32 matches 4.32.0
func FuzzyMatchVersion(version1, version2 string) bool {
	// Split versions into parts
	parts1 := splitVersion(version1)
	parts2 := splitVersion(version2)

	// Get minimum length to compare
	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	// Compare up to the minimum length
	for i := 0; i < minLen; i++ {
		if parts1[i] != parts2[i] {
			return false
		}
	}

	// Check if the longer version has only zeros in the extra parts
	if len(parts1) > len(parts2) {
		return allZeros(parts1[len(parts2):])
	}
	return allZeros(parts2[len(parts1):])
}

// splitVersion splits a version string into parts
func splitVersion(version string) []string {
	return strings.Split(version, ".")
}

// allZeros checks if all parts are "0"
func allZeros(parts []string) bool {
	for _, part := range parts {
		if part != "0" {
			return false
		}
	}
	return true
}
