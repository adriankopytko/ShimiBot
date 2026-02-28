package tools

import (
	"encoding/json"
	"strings"
)

func NormalizeJSONArguments(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}", true
	}

	if json.Valid([]byte(trimmed)) {
		return trimmed, true
	}

	if extracted, ok := extractBalancedJSON(trimmed, '{', '}'); ok {
		if json.Valid([]byte(extracted)) {
			return extracted, true
		}
	}

	if extracted, ok := extractBalancedJSON(trimmed, '[', ']'); ok {
		if json.Valid([]byte(extracted)) {
			return extracted, true
		}
	}

	return "", false
}

func extractBalancedJSON(input string, open, close rune) (string, bool) {
	start := -1
	depth := 0
	inString := false
	escaped := false

	for index, r := range input {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}

		if r == '"' {
			inString = true
			continue
		}

		if r == open {
			if start == -1 {
				start = index
			}
			depth++
			continue
		}

		if r == close {
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				return strings.TrimSpace(input[start : index+1]), true
			}
		}
	}

	return "", false
}
