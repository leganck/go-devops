package deploy

import (
	"strings"
)

func FuzzyMatchVersion(inputVersion, apiVersion string) bool {
	cleanAPI := cleanVersion(apiVersion)
	if inputVersion == cleanAPI {
		return true
	}
	inputHasGroup := hasGroup(inputVersion)
	apiHasGroup := hasGroup(cleanAPI)
	if inputHasGroup {
		if extractGroup(inputVersion) != extractGroup(cleanAPI) {
			return false
		}
	} else if apiHasGroup {
		return false
	}
	parts1 := strings.Split(inputVersion, ".")
	parts2 := strings.Split(cleanAPI, ".")
	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}
	for i := 0; i < minLen; i++ {
		if !strings.HasPrefix(parts2[i], parts1[i]) {
			return false
		}
	}
	if len(parts1) <= len(parts2) {
		return true
	}
	for _, part := range parts1[len(parts2):] {
		if part != "0" {
			return false
		}
	}
	return true
}

func cleanVersion(version string) string {
	version = strings.TrimSuffix(version, ".jar")
	if idx := strings.Index(version, "-SNAPSHOT"); idx > 0 {
		version = version[:idx]
	}
	return version
}

func hasGroup(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		return strings.Contains(parts[len(parts)-1], "-")
	}
	return false
}

func extractGroup(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if idx := strings.Index(last, "-"); idx >= 0 {
			return last[idx+1:]
		}
	}
	return ""
}
