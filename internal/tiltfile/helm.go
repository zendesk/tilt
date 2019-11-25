package tiltfile

import (
	"regexp"
	"strings"
)

// The helm template command outputs predictable yaml with a "Source:" comment,
// so take advantage of that.
const helmSeparator = "---\n"

var helmTestYAMLMatcher = regexp.MustCompile("^# Source: .*/tests/")

func filterHelmTestYAML(resourceBlob string) string {
	result := []string{}
	resources := strings.Split(resourceBlob, helmSeparator)
	for _, resource := range resources {
		if isHelmTestYAML(resource) {
			continue
		}

		result = append(result, resource)
	}
	return strings.Join(result, helmSeparator)
}

func isHelmTestYAML(resource string) bool {
	lines := strings.Split(resource, "\n")
	for _, line := range lines {
		if helmTestYAMLMatcher.MatchString(line) {
			return true
		}
	}
	return false
}

type helmVersion int

const (
	unknownHelmVersion helmVersion = iota
	helmV2
	helmV3
)

func parseVersion(version string) helmVersion {
	if strings.HasPrefix(version, "v3") {
		return helmV3
	} else if strings.HasPrefix(version, "v2") {
		return helmV2
	}

	return unknownHelmVersion
}
