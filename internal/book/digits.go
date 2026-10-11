package book

import (
	"fmt"
	"regexp"
	"strings"
)

// SMS digits link an account to the last digits banks print in their messages
// ("Card XX4275", "A/C XXXXX810353" → 4275, 0353).

var digitsRe = regexp.MustCompile(`^\d{3,6}$`)

// CleanDigits trims, de-duplicates and validates a list of SMS digits.
func CleanDigits(in []string) ([]string, error) {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !digitsRe.MatchString(s) {
			return nil, fmt.Errorf("SMS digits must be 3 to 6 digits, not %q", s)
		}
		if !contains(out, s) {
			out = append(out, s)
		}
	}
	return out, nil
}

// DigitsFree reports an error when another account already uses one of digits.
func (d *Data) DigitsFree(digits []string, self string) error {
	for _, a := range d.Accounts {
		if a.ID == self {
			continue
		}
		for _, x := range a.SMSDigits {
			if contains(digits, x) {
				return fmt.Errorf("SMS digits %s already belong to %s", x, a.Name)
			}
		}
	}
	return nil
}

// AccountByDigits returns the account whose SMS digits match the end of digits
// (an SMS may print more digits than you stored, e.g. 810353 vs 0353).
func (d *Data) AccountByDigits(digits string) (string, bool) {
	if digits == "" {
		return "", false
	}
	for _, a := range d.Accounts {
		for _, x := range a.SMSDigits {
			if strings.HasSuffix(digits, x) || strings.HasSuffix(x, digits) {
				return a.ID, true
			}
		}
	}
	return "", false
}
