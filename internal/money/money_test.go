package money

import "testing"

func TestParse(t *testing.T) {
	ok := map[string]Amount{
		"0": 0, "12": 1200, "12.5": 1250, "12.05": 1205, "-12.05": -1205, "₹ 1,23,456.78": 12345678,
		"(10)": -1000, "+3": 300, ".5": 50, "7.": 700, "Rs 99": 9900,
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "1.234", "1e3", "--1", "12.3.4"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[Amount]string{0: "₹0.00", 5: "₹0.05", 100000: "₹1,000.00", 12345678: "₹1,23,456.78", -1234567890: "-₹1,23,45,678.90"}
	for a, want := range cases {
		if got := a.Format(); got != want {
			t.Errorf("%d.Format() = %q; want %q", a, got, want)
		}
	}
	if s := Amount(-5).String(); s != "-0.05" {
		t.Errorf("String = %q", s)
	}
}

func TestMulDiv(t *testing.T) {
	if got := Amount(1200000).MulDiv(12, 52); got != 276923 { // ₹12,000/month as a week
		t.Errorf("MulDiv = %d", got)
	}
	if got := Amount(-100).MulDiv(1, 3); got != -33 {
		t.Errorf("MulDiv negative = %d", got)
	}
}
