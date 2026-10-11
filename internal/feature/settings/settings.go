// Package settings is the Settings feature: default category period, week/month/
// year start, look-ahead days, theme, amount privacy, menu order and the inbox's
// fallback categories. Sign-in passwords live in
// .env and are not editable here.
package settings

import (
	"fmt"
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
	if s.UpcomingDays < 1 || s.UpcomingDays > 365 {
		return fmt.Errorf("upcoming days must be 1–365")
	}
	if s.NavOrder != nil {
		order, err := navOrder(s.NavOrder)
		if err != nil {
			return err
		}
		s.NavOrder = order
	}
	switch s.Theme {
	case "auto", "light", "dark":
	default:
		return fmt.Errorf("theme must be auto, light or dark")
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
