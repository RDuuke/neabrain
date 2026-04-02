package domain

import "strings"

const (
	disclosureLow    = "low"
	disclosureMedium = "medium"
	disclosureHigh   = "high"
)

func normalizeDisclosureLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case disclosureLow:
		return disclosureLow
	case disclosureMedium:
		return disclosureMedium
	case disclosureHigh:
		return disclosureHigh
	default:
		return ""
	}
}

func isObservationVisible(tags []string, disclosureLevel string) bool {
	level := normalizeDisclosureLevel(disclosureLevel)
	if level == "" || level == disclosureHigh {
		return true
	}

	hasPrivate := hasTag(tags, "private")
	hasSensitive := hasTag(tags, "sensitive")

	if level == disclosureLow {
		return !hasPrivate && !hasSensitive
	}
	if level == disclosureMedium {
		return !hasPrivate
	}
	return true
}

func filterObservationsByDisclosure(observations []Observation, disclosureLevel string) []Observation {
	if normalizeDisclosureLevel(disclosureLevel) == "" {
		return observations
	}

	filtered := make([]Observation, 0, len(observations))
	for _, observation := range observations {
		if isObservationVisible(observation.Tags, disclosureLevel) {
			filtered = append(filtered, observation)
		}
	}
	return filtered
}

func hasTag(tags []string, needle string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), needle) {
			return true
		}
	}
	return false
}
