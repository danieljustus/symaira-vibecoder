package engine

import "testing"

func TestPredicateHolds(t *testing.T) {
	cases := []struct {
		val  int
		when string
		want bool
	}{
		{0, "==0", true}, {1, "==0", false},
		{3, ">0", true}, {0, ">0", false},
		{5, ">=5", true}, {4, ">=5", false},
		{2, "<=2", true}, {3, "<=2", false},
		{0, "!=0", false}, {1, "!=0", true},
		{0, "clean", true}, {2, "clean", false},
		{2, "changed", true}, {0, "changed", false},
		{7, "7", true}, {7, "8", false},
		{1, "garbage", false},
	}
	for _, c := range cases {
		if got := predicateHolds(c.val, c.when); got != c.want {
			t.Errorf("predicateHolds(%d,%q)=%v want %v", c.val, c.when, got, c.want)
		}
	}
}

func TestSensorNamesRegistered(t *testing.T) {
	if len(SensorNames()) < 3 {
		t.Fatalf("expected the core sensors to be registered, got %v", SensorNames())
	}
}
