package screen

import "testing"

func TestAtoi(t *testing.T) {
	for in, want := range map[string]int{"4242": 4242, "": 0, "12a": 0, "0": 0} {
		if got := atoi(in); got != want {
			t.Errorf("atoi(%q) = %d, want %d", in, got, want)
		}
	}
}
