package review

import (
	"fmt"
	"testing"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/model"
)

func TestPageRules(t *testing.T) {
	d := &book.Data{Categories: []model.Category{{ID: "food", Name: "Food", Kind: model.KindExpense}}}
	for i := 0; i < 1200; i++ {
		d.Rules = append(d.Rules, model.Rule{ID: fmt.Sprint("r", i), Match: fmt.Sprintf("merchant %04d", i), Title: "Shop", Category: "misc", Uses: i % 7})
	}
	d.Rules = append(d.Rules, model.Rule{ID: "swiggy", Match: "swiggy", Title: "Swiggy", Category: "food", Uses: 3})
	d.Reindex()

	p := PageRules(d, "", 0, 50)
	if p.Total != 1201 || len(p.Items) != 50 || p.Items[0].Uses != 6 {
		t.Errorf("first page: total %d, %d items, top uses %d", p.Total, len(p.Items), p.Items[0].Uses)
	}
	if last := PageRules(d, "", 1200, 50); len(last.Items) != 1 || last.Offset != 1200 {
		t.Errorf("last page = %+v", last)
	}
	if past := PageRules(d, "", 5000, 50); len(past.Items) != 0 || past.Offset != 1201 {
		t.Errorf("past the end = %+v", past)
	}
	if big := PageRules(d, "", 0, 10_000); big.Limit != 200 || len(big.Items) != 200 {
		t.Errorf("limit must be capped, got %d", big.Limit)
	}
	if f := PageRules(d, "FOOD", 0, 50); f.Total != 1 || f.Items[0].ID != "swiggy" {
		t.Errorf("search by category name = %+v", f)
	}
	if f := PageRules(d, "merchant 01", 0, 50); f.Total != 100 {
		t.Errorf("search by merchant = %d", f.Total)
	}
}
