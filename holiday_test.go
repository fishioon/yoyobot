package main

import "testing"

func TestParseAmount(t *testing.T) {
	f := func(t *testing.T, s string, expect int64) {
		if amount, _ := ParseAmount(s); amount != expect {
			t.Errorf("%s got %d expect %d", s, amount, expect)
		}
	}
	f(t, "1", 2)
	f(t, "3.5", 7)
	f(t, "-0.5", -1)
	f(t, "-5 天", -10)
	f(t, "0", 0)
	f(t, "测试时", 0)
	f(t, "三天", 0)
}

func TestFormatAmount(t *testing.T) {
	f := func(t *testing.T, amount int64, expect string) {
		if s := FormatAmount(amount); s != expect {
			t.Errorf("%d got %s expect %s", amount, s, expect)
		}
	}
	f(t, 0, "0")
	f(t, 1, "0.5")
	f(t, 12, "6")
	f(t, 21, "10.5")
	f(t, -0, "0")
	f(t, -1, "-0.5")
}
