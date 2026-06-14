package auth

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// e164Re validates a normalized E.164 phone number: + followed by 7–15 digits.
var e164Re = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// NormalizePhone normalizes a phone number to E.164 format.
//
// Accepted input forms:
//   - Already E.164: "+94771234567"
//   - Sri Lanka local (leading 0): "0771234567" → "+94771234567"
//   - Country code without +: "94771234567" → "+94771234567"
//   - Whitespace / hyphens / parentheses are stripped before parsing.
//
// Returns an error if the number cannot be expressed as a valid E.164 string.
func NormalizePhone(input string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' || r == '(' || r == ')' {
			return -1
		}
		return r
	}, input)

	if cleaned == "" {
		return "", fmt.Errorf("phone: empty after stripping whitespace")
	}

	// Already E.164 format.
	if strings.HasPrefix(cleaned, "+") {
		if !e164Re.MatchString(cleaned) {
			return "", fmt.Errorf("phone: %q is not a valid E.164 number", input)
		}
		return cleaned, nil
	}

	// Sri Lanka local format: leading 0 with at least 9 digits total.
	if strings.HasPrefix(cleaned, "0") && len(cleaned) >= 9 {
		candidate := "+94" + cleaned[1:]
		if !e164Re.MatchString(candidate) {
			return "", fmt.Errorf("phone: %q cannot be normalized to E.164", input)
		}
		return candidate, nil
	}

	// Bare country code without +.
	candidate := "+" + cleaned
	if e164Re.MatchString(candidate) {
		return candidate, nil
	}

	return "", fmt.Errorf("phone: %q cannot be recognized as a phone number", input)
}
