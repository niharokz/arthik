package inbox

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"gitlab.com/niharokz/arthik/internal/money"
)

//go:embed sms_rules.yaml
var defaultRules []byte

// RulesFile is the optional user rules file inside the dataset (same layout as
// sms_rules.yaml; its rules are tried before the built-in ones).
const RulesFile = "inbox/sms_rules.md"

// Rule kinds (what money did).
const (
	KindExpense    = "expense"
	KindIncome     = "income"
	KindTransferIn = "transfer_in" // money arriving on a card or loan (a payment)
	KindAuto       = "auto"
)

type ruleSpec struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Match   string `yaml:"match"`
	Generic bool   `yaml:"generic"`
	Enabled *bool  `yaml:"enabled"`
}

type rulesDoc struct {
	Macros        map[string]string `yaml:"macros"`
	DateLayouts   []string          `yaml:"date_layouts"`
	Ignore        []string          `yaml:"ignore"`
	CreditWords   []string          `yaml:"credit_words"`
	DebitWords    []string          `yaml:"debit_words"`
	FindDigits    string            `yaml:"find_digits"`
	FindMerchant  []string          `yaml:"find_merchant"` // tried in order
	MerchantStrip []string          `yaml:"merchant_strip"`
	Rules         []ruleSpec        `yaml:"rules"`
	Disable       []string          `yaml:"disable"` // built-in rule names to switch off
}

type smsRule struct {
	name    string
	kind    string
	generic bool
	re      *regexp.Regexp
}

// Parser turns bank SMS text into parsed transactions using data-driven rules.
type Parser struct {
	rules        []smsRule
	ignore       []*regexp.Regexp
	strip        []*regexp.Regexp
	credit       []string
	debit        []string
	layouts      []string
	findDigits   *regexp.Regexp
	findMerchant []*regexp.Regexp
	dateRe       *regexp.Regexp
	timeRe       *regexp.Regexp
}

// Parsed is what one SMS says.
type Parsed struct {
	Rule      string
	Generic   bool // read by the catch-all rule: worth a second look
	Kind      string
	Amount    money.Amount
	Digits    string
	Merchant  string
	Date      string // YYYY-MM-DD ("" = use the message time)
	Time      string // HH:MM
	Balance   money.Amount
	Available money.Amount
}

// NewParser builds a parser from the built-in rules plus the user's file (may be nil).
func NewParser(user []byte) (*Parser, error) {
	var def rulesDoc
	if err := yaml.Unmarshal(defaultRules, &def); err != nil {
		return nil, fmt.Errorf("built-in SMS rules: %w", err)
	}
	var usr rulesDoc
	if len(strings.TrimSpace(string(user))) > 0 {
		if err := yaml.Unmarshal(user, &usr); err != nil {
			return nil, fmt.Errorf("%s: %w", RulesFile, err)
		}
	}
	macros := map[string]string{}
	for k, v := range def.Macros {
		macros[k] = v
	}
	for k, v := range usr.Macros {
		macros[k] = v
	}
	expand := func(s string) string {
		for k, v := range macros {
			s = strings.ReplaceAll(s, "{"+k+"}", "(?:"+v+")")
		}
		return s
	}
	p := &Parser{
		credit:  append(usr.CreditWords, def.CreditWords...),
		debit:   append(usr.DebitWords, def.DebitWords...),
		layouts: append(usr.DateLayouts, def.DateLayouts...),
	}
	compile := func(what, src string) (*regexp.Regexp, error) {
		re, err := regexp.Compile(expand(src))
		if err != nil {
			return nil, fmt.Errorf("SMS %s %q: %w", what, src, err)
		}
		return re, nil
	}
	var err error
	for _, src := range append(usr.Ignore, def.Ignore...) {
		re, err := compile("ignore pattern", src)
		if err != nil {
			return nil, err
		}
		p.ignore = append(p.ignore, re)
	}
	for _, src := range append(usr.MerchantStrip, def.MerchantStrip...) {
		re, err := compile("merchant_strip pattern", src)
		if err != nil {
			return nil, err
		}
		p.strip = append(p.strip, re)
	}
	pick := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	if p.findDigits, err = compile("find_digits", pick(usr.FindDigits, def.FindDigits)); err != nil {
		return nil, err
	}
	for _, src := range append(usr.FindMerchant, def.FindMerchant...) {
		re, err := compile("find_merchant", src)
		if err != nil {
			return nil, err
		}
		p.findMerchant = append(p.findMerchant, re)
	}
	if p.dateRe, err = compile("date macro", "{date}"); err != nil {
		return nil, err
	}
	if p.timeRe, err = compile("time macro", `\b{time}`); err != nil {
		return nil, err
	}
	off := map[string]bool{}
	for _, n := range usr.Disable {
		off[n] = true
	}
	for _, src := range [][]ruleSpec{usr.Rules, def.Rules} {
		for _, r := range src {
			if off[r.Name] || (r.Enabled != nil && !*r.Enabled) {
				continue
			}
			re, err := compile("rule "+r.Name, r.Match)
			if err != nil {
				return nil, err
			}
			if !contains(re.SubexpNames(), "amount") {
				return nil, fmt.Errorf("SMS rule %s has no (?P<amount>…) group", r.Name)
			}
			kind := r.Type
			switch kind {
			case KindExpense, KindIncome, KindTransferIn, KindAuto:
			case "":
				kind = KindAuto
			default:
				return nil, fmt.Errorf("SMS rule %s: unknown type %q", r.Name, r.Type)
			}
			p.rules = append(p.rules, smsRule{name: r.Name, kind: kind, generic: r.Generic, re: re})
		}
	}
	return p, nil
}

// LoadParser reads the dataset's user rules (if any) and builds a parser.
func LoadParser(dir string) (*Parser, error) {
	b, err := os.ReadFile(filepath.Join(dir, RulesFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return NewParser(b)
}

// Parse reads one SMS. ok=false means it is not a transaction we can use.
func (p *Parser) Parse(body string) (Parsed, bool) {
	text := strings.Join(strings.Fields(body), " ")
	for _, re := range p.ignore {
		if re.MatchString(text) {
			return Parsed{}, false
		}
	}
	for _, r := range p.rules {
		m := r.re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		g := func(name string) string {
			if i := r.re.SubexpIndex(name); i > 0 && i < len(m) {
				return strings.TrimSpace(m[i])
			}
			return ""
		}
		out := Parsed{Rule: r.name, Generic: r.generic, Kind: r.kind, Digits: g("digits"), Merchant: p.cleanMerchant(g("merchant"))}
		amt, err := parseAmount(g("amount"))
		if err != nil || amt <= 0 {
			continue
		}
		out.Amount = amt
		out.Balance, _ = parseAmount(g("balance"))
		out.Available, _ = parseAmount(g("available"))
		if out.Kind == KindAuto {
			out.Kind = p.direction(g("dir"), text)
			if out.Kind == "" {
				continue // amount without a direction: a promo, a statement, …
			}
		}
		date, tm := g("date"), g("time")
		if r.generic {
			if out.Digits == "" {
				if dm := p.findDigits.FindStringSubmatch(text); dm != nil {
					out.Digits = dm[1]
				}
			}
			for _, re := range p.findMerchant {
				if out.Merchant != "" {
					break
				}
				if mm := re.FindStringSubmatch(text); mm != nil {
					out.Merchant = p.cleanMerchant(mm[1])
				}
			}
			if date == "" {
				date = p.dateRe.FindString(text)
			}
			if tm == "" && date != "" {
				tm = p.timeRe.FindString(text[strings.Index(text, date)+len(date):])
			}
		}
		out.Date = p.parseDate(date)
		out.Time = parseClock(tm)
		return out, true
	}
	return Parsed{}, false
}

// direction decides income/expense from the dir group or, failing that, the words.
func (p *Parser) direction(dir, text string) string {
	low := strings.ToLower(text)
	if dir != "" {
		low = strings.ToLower(dir)
	}
	first := func(words []string) int {
		best := -1
		for _, w := range words {
			if i := wordIndex(low, strings.ToLower(w)); i >= 0 && (best < 0 || i < best) {
				best = i
			}
		}
		return best
	}
	c, d := first(p.credit), first(p.debit)
	switch {
	case c < 0 && d < 0:
		return ""
	case d < 0 || (c >= 0 && c < d):
		return KindIncome
	default:
		return KindExpense
	}
}

func (p *Parser) cleanMerchant(s string) string {
	for _, re := range p.strip {
		s = re.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}

func (p *Parser) parseDate(s string) string {
	if s == "" {
		return ""
	}
	s = strings.NewReplacer("/", "-", ".", "-", " ", "-", ",", "").Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	for _, l := range p.layouts {
		if t, err := time.Parse(l, s); err == nil {
			if t.Year() < 100 {
				t = t.AddDate(2000, 0, 0)
			}
			return t.Format("2006-01-02")
		}
		// Month names in SMS are often upper case ("09OCT26").
		if t, err := time.Parse(l, titleMonth(s)); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

var clockRe = regexp.MustCompile(`(?i)^(\d{1,2}):(\d{2})(?::\d{2})?\s*([ap])?\.?m?\.?$`)

// parseClock turns "09:57:36 pm" / "20:34:36" into "21:57" / "20:34".
func parseClock(s string) string {
	m := clockRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1]) // not Sscan: it reads "09" as octal
	min, _ := strconv.Atoi(m[2])
	switch strings.ToLower(m[3]) {
	case "p":
		if h < 12 {
			h += 12
		}
	case "a":
		if h == 12 {
			h = 0
		}
	}
	if h > 23 || min > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", h, min)
}

func parseAmount(s string) (money.Amount, error) {
	if s == "" {
		return 0, nil
	}
	return money.Parse(strings.ReplaceAll(s, ",", ""))
}

var monthRe = regexp.MustCompile(`(?i)[a-z]{3}`)

func titleMonth(s string) string {
	return monthRe.ReplaceAllStringFunc(s, func(m string) string { return strings.ToUpper(m[:1]) + strings.ToLower(m[1:]) })
}

// wordIndex finds w in s at word boundaries (-1 when absent).
func wordIndex(s, w string) int {
	for from := 0; ; {
		i := strings.Index(s[from:], w)
		if i < 0 {
			return -1
		}
		i += from
		before := i == 0 || !isAlnum(s[i-1])
		after := i+len(w) >= len(s) || !isAlnum(s[i+len(w)])
		if before && after {
			return i
		}
		from = i + 1
	}
}

func isAlnum(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
