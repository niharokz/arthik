// Package money is Arthik's exact-decimal amount type.
//
// Every amount is held as an integer number of paise (1/100 rupee), so sums are
// always exact — no float rounding ever touches a balance. On disk and in JSON an
// amount is a plain decimal string such as "1234.50" or "-99.00".
package money

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Amount is a signed amount in paise.
type Amount int64

// Parse reads a human amount: "1234", "1,23,456.7", "₹ 500", "-12.05", "(12.05)".
// More than two decimal places is an error rather than a silent rounding.
func Parse(s string) (Amount, error) {
	raw := s
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("₹", "", ",", "", " ", "", "_", "", "INR", "", "Rs.", "", "Rs", "").Replace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, s[1:len(s)-1]
	}
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = !neg, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" && frac == "" {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	if hasDot && len(frac) > 2 {
		return 0, fmt.Errorf("amount %q has more than 2 decimal places", raw)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	if whole == "" {
		whole = "0"
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid amount %q", raw)
		}
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w > 9e15 {
		return 0, fmt.Errorf("amount %q is out of range", raw)
	}
	f, _ := strconv.ParseInt(frac, 10, 64)
	v := Amount(w*100 + f)
	if neg {
		v = -v
	}
	return v, nil
}

// MustParse is Parse for constants in tests and the demo seeder.
func MustParse(s string) Amount {
	a, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return a
}

// FromRupees builds an amount from whole rupees.
func FromRupees(r int64) Amount { return Amount(r * 100) }

// String is the plain storage form: "1234.50", "-0.05".
func (a Amount) String() string {
	sign := ""
	v := int64(a)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// Format is the display form with Indian digit grouping: "₹1,23,456.78".
func (a Amount) Format() string {
	sign := ""
	v := int64(a)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s₹%s.%02d", sign, groupIndian(v/100), v%100)
}

// groupIndian inserts commas as 12,34,567 (last three digits, then pairs).
func groupIndian(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	head, tail := s[:len(s)-3], s[len(s)-3:]
	var parts []string
	for len(head) > 2 {
		parts = append([]string{head[len(head)-2:]}, parts...)
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append([]string{head}, parts...)
	}
	return strings.Join(parts, ",") + "," + tail
}

// Abs returns |a|.
func (a Amount) Abs() Amount {
	if a < 0 {
		return -a
	}
	return a
}

// MulDiv scales a by num/den with half-away-from-zero rounding (used to convert a
// budget between periods). den must be positive.
func (a Amount) MulDiv(num, den int64) Amount {
	p := int64(a) * num
	q := p / den
	r := p % den
	if r*2 >= den {
		q++
	} else if r*2 <= -den {
		q--
	}
	return Amount(q)
}

// IsZero reports a == 0.
func (a Amount) IsZero() bool { return a == 0 }

// MarshalYAML writes the amount as a quoted decimal string.
func (a Amount) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.SingleQuotedStyle, Value: a.String()}, nil
}

// UnmarshalYAML accepts a quoted string or a bare number (Obsidian edits).
func (a *Amount) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: amount must be a number", n.Line)
	}
	if n.Tag == "!!null" || strings.TrimSpace(n.Value) == "" {
		*a = 0
		return nil
	}
	v, err := Parse(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %v", n.Line, err)
	}
	*a = v
	return nil
}

// MarshalJSON writes "1234.50" (a string, so JavaScript never sees a float).
func (a Amount) MarshalJSON() ([]byte, error) { return json.Marshal(a.String()) }

// UnmarshalJSON accepts a string or a number.
func (a *Amount) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` {
		*a = 0
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
	}
	v, err := Parse(s)
	if err != nil {
		return err
	}
	*a = v
	return nil
}
