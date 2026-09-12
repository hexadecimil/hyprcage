package screen

import "testing"

func TestFittedSize(t *testing.T) {
	if w, h := fittedSize(1280, 800, [4]int{0, 0, 0, 0}); w != 1280 || h != 800 {
		t.Errorf("no reserved area: %dx%d", w, h)
	}
	if w, h := fittedSize(1280, 800, [4]int{0, 26, 0, 0}); w != 1280 || h != 826 {
		t.Errorf("top bar: %dx%d", w, h)
	}
	if w, h := fittedSize(1280, 800, [4]int{40, 0, 0, 30}); w != 1320 || h != 830 {
		t.Errorf("left dock + bottom bar: %dx%d", w, h)
	}
}
