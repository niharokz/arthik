// Package settings is the Settings feature: default category period, week/month/
// year start, look-ahead days, theme and look (palette, cards, colours, charts),
// the phone's daily alert, amount privacy, menu order, home layout and
// the inbox's fallback categories. Sign-in passwords live in
// .env and are not editable here.
package settings

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/period"
)

// Register adds the route.
func Register(rt *httpx.Router) {
	rt.Handle("PUT /api/settings", func(c *httpx.Ctx) error {
		var in model.Settings
		if err := c.Decode(&in); err != nil {
			return err
		}
		if err := validate(&in); err != nil {
			return err
		}
		err := c.Book.Mutate(func(d *book.Data, ch *book.Change) error {
			in.Extra = d.Settings.Extra // keep hand-added keys
			if in.NavOrder == nil {
				in.NavOrder = d.Settings.NavOrder // a form without the menu order keeps it
			}
			if in.Home == nil {
				in.Home = d.Settings.Home // likewise the home layout
			}
			if in.CheckDays == 0 {
				in.CheckDays = d.Settings.CheckDays
			}
			if in.Look == nil {
				in.Look = d.Settings.Look // a form without the look keeps it
			}
			if in.Alerts == nil {
				in.Alerts = d.Settings.Alerts // likewise the daily alert
			}
			in.Home = homeRefs(d, in.Home)
			for _, c := range []struct{ id, kind string }{{in.InboxExpenseCategory, model.KindExpense}, {in.InboxIncomeCategory, model.KindIncome}} {
				if cat, ok := d.Category(c.id); c.id != "" && (!ok || (cat.Kind != c.kind && cat.Kind != model.KindBoth)) {
					return fmt.Errorf("the inbox fallback category %q is not an %s category", c.id, c.kind)
				}
			}
			d.Settings = in
			ch.Settings = true
			return nil
		})
		if err != nil {
			return err
		}
		return c.OK(map[string]bool{"ok": true})
	})
}

func validate(s *model.Settings) error {
	k := period.Kind(s.DefaultPeriod)
	if !k.Valid() || k == period.Custom {
		return fmt.Errorf("default period must be weekly, monthly, quarterly, halfyearly or yearly")
	}
	ok := false
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		if strings.EqualFold(wd.String(), s.WeekStart) {
			s.WeekStart, ok = strings.ToLower(wd.String()), true
		}
	}
	if !ok {
		return fmt.Errorf("week start must be a weekday name")
	}
	if s.MonthStartDay < 1 || s.MonthStartDay > 28 {
		return fmt.Errorf("month start day must be 1–28")
	}
	if s.YearStartMonth < 1 || s.YearStartMonth > 12 {
		return fmt.Errorf("year start month must be 1–12")
	}
	if s.InboxSMSTime != "" {
		t, err := time.Parse("15:04", s.InboxSMSTime)
		if err != nil {
			return fmt.Errorf("SMS → drafts time must be HH:MM (24-hour), or empty for “as soon as they arrive”")
		}
		s.InboxSMSTime = t.Format("15:04")
	}
	if s.UpcomingDays < 1 || s.UpcomingDays > 365 {
		return fmt.Errorf("upcoming days must be 1–365")
	}
	if s.CheckDays < 0 || s.CheckDays > 365 {
		return fmt.Errorf("balance check days must be 1–365")
	}
	if s.NavOrder != nil {
		order, err := navOrder(s.NavOrder)
		if err != nil {
			return err
		}
		s.NavOrder = order
	}
	if s.Home != nil {
		home, err := homeLayout(s.Home)
		if err != nil {
			return err
		}
		s.Home = home
	}
	switch s.Theme {
	case "auto", "light", "dark":
	default:
		return fmt.Errorf("theme must be auto, light or dark")
	}
	if s.Look != nil {
		if err := look(s.Look); err != nil {
			return err
		}
	}
	if s.Alerts != nil {
		if err := alerts(s.Alerts); err != nil {
			return err
		}
	}
	return nil
}

// look checks the palette, card style, colours and chart types.
func look(l *model.Look) error {
	if !slices.Contains(model.Palettes, l.Palette) {
		return fmt.Errorf("theme must be one of %s", strings.Join(model.Palettes, ", "))
	}
	if !slices.Contains(model.CardStyles, l.Cards) {
		return fmt.Errorf("card background must be one of %s", strings.Join(model.CardStyles, ", "))
	}
	for k, v := range l.Colors {
		if !slices.Contains(model.LookColors, k) {
			return fmt.Errorf("unknown colour %q", k)
		}
		if !model.HexColor(v) {
			return fmt.Errorf("colour %s must be #rrggbb", k)
		}
		l.Colors[k] = strings.ToLower(v)
	}
	for k, v := range l.Charts {
		kinds, ok := model.Charts[k]
		if !ok {
			return fmt.Errorf("unknown chart %q", k)
		}
		if !slices.Contains(kinds, v) {
			return fmt.Errorf("the %s chart can be %s", k, strings.Join(kinds, ", "))
		}
	}
	return nil
}

// alerts checks the daily alert time and threshold.
func alerts(a *model.Alerts) error {
	t, err := time.Parse("15:04", a.Time)
	if err != nil {
		return fmt.Errorf("alert time must be HH:MM (24-hour)")
	}
	a.Time = t.Format("15:04")
	if a.Count < 0 || a.Count > 9999 {
		return fmt.Errorf("alert count must be 0–9999 (0 = off)")
	}
	return nil
}

// navOrder checks a menu order and appends screens it leaves out (in default order).
func navOrder(in []string) ([]string, error) {
	out, unknown := model.NormNav(in)
	if unknown != "" {
		return nil, fmt.Errorf("unknown menu item %q", unknown)
	}
	return out, nil
}

// homeLayout checks the home sections: known types, each built-in once, a ref on
// custom sections, at most 40.
func homeLayout(in []model.HomeSection) ([]model.HomeSection, error) {
	if len(in) > 40 {
		return nil, fmt.Errorf("the home screen takes at most 40 sections")
	}
	builtin, custom := map[string]bool{}, map[string]bool{}
	for _, t := range model.HomeBuiltins {
		builtin[t] = true
	}
	for _, t := range model.HomeCustom {
		custom[t] = true
	}
	seen := map[string]bool{}
	out := []model.HomeSection{}
	for _, h := range in {
		h.Type, h.Ref, h.Title = strings.TrimSpace(h.Type), strings.TrimSpace(h.Ref), strings.TrimSpace(h.Title)
		switch {
		case builtin[h.Type]:
			h.Ref = ""
		case custom[h.Type]:
			if h.Ref == "" {
				return nil, fmt.Errorf("a %s section needs a %s", h.Type, h.Type)
			}
		default:
			return nil, fmt.Errorf("unknown home section %q", h.Type)
		}
		if h.Style != "" && !slices.Contains(model.CardStyles, h.Style) {
			return nil, fmt.Errorf("unknown card background %q", h.Style)
		}
		if len(h.Title) > 60 {
			return nil, fmt.Errorf("section titles are at most 60 characters")
		}
		key := h.Type + "/" + h.Ref
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
	}
	return out, nil
}

// homeRefs drops custom sections whose account, category or label no longer exists.
func homeRefs(d *book.Data, home []model.HomeSection) []model.HomeSection {
	if home == nil {
		return nil
	}
	out := []model.HomeSection{}
	for _, h := range home {
		ok := true
		switch h.Type {
		case "account":
			_, ok = d.Account(h.Ref)
		case "category":
			_, ok = d.Category(h.Ref)
		case "label":
			_, ok = d.Label(h.Ref)
		}
		if ok {
			out = append(out, h)
		}
	}
	return out
}
