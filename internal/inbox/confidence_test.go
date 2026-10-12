package inbox

import (
	"reflect"
	"testing"

	"gitlab.com/niharokz/arthik/internal/model"
)

func TestConfidence(t *testing.T) {
	base := model.Draft{Type: model.TxExpense, Account: "hdfc", Category: "food", Via: model.ViaLearned, Source: SourceSMS}
	with := func(f func(*model.Draft)) model.Draft { x := base; f(&x); return x }
	cases := []struct {
		name   string
		x      model.Draft
		dup    bool
		warned bool
		want   string
		checks []string
	}{
		{"learned", base, false, false, High, nil},
		{"past", with(func(x *model.Draft) { x.Via = model.ViaPast }), false, false, High, nil},
		{"edited fallback", with(func(x *model.Draft) { x.Via, x.Edited = model.ViaDefault, true }), false, false, High, nil},
		{"kronos", with(func(x *model.Draft) { x.Via, x.Source = model.ViaSource, SourceKronos }), false, false, Medium, []string{"Category taken from Kronos"}},
		{"ai", with(func(x *model.Draft) { x.Via = model.ViaAI }), false, false, Medium, []string{"Category suggested by the AI"}},
		{"fallback", with(func(x *model.Draft) { x.Via = model.ViaDefault }), false, false, Low, []string{"Category is only the fallback"}},
		{"no category", with(func(x *model.Draft) { x.Via, x.Category = "", "" }), false, false, Low, []string{"No category yet"}},
		{"no account", with(func(x *model.Draft) { x.Account = "" }), false, false, Low, []string{"Account not known"}},
		{"duplicate", base, true, false, Low, []string{"May already be recorded"}},
		{"warned", base, false, true, Medium, []string{"See the warnings"}},
		{"ai then unknown account stays low", with(func(x *model.Draft) { x.Via, x.Account = model.ViaAI, "" }), false, true, Low,
			[]string{"Category suggested by the AI", "Account not known", "See the warnings"}},
		{"transfer needs both accounts", model.Draft{Type: model.TxTransfer, Account: "idfc"}, false, false, Low, []string{"Account not known"}},
	}
	for _, c := range cases {
		got, checks := confidence(c.x, c.dup, c.warned)
		if got != c.want || !reflect.DeepEqual(checks, c.checks) {
			t.Errorf("%s: got %s %q, want %s %q", c.name, got, checks, c.want, c.checks)
		}
	}
}
