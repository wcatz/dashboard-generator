package generator

import "testing"

func TestLayoutPlace(t *testing.T) {
	le := NewLayoutEngine()

	// First panel at origin
	x, y := le.Place(3, 4)
	if x != 0 || y != 0 {
		t.Errorf("Place(3,4) = (%d,%d), want (0,0)", x, y)
	}

	// Second panel next to first
	x, y = le.Place(3, 4)
	if x != 3 || y != 0 {
		t.Errorf("Place(3,4) = (%d,%d), want (3,0)", x, y)
	}

	// Fill rest of row (total so far: 6 of 24)
	x, y = le.Place(18, 4)
	if x != 6 || y != 0 {
		t.Errorf("Place(18,4) = (%d,%d), want (6,0)", x, y)
	}

	// Next panel wraps to row 2
	x, y = le.Place(12, 7)
	if x != 0 || y != 4 {
		t.Errorf("Place(12,7) = (%d,%d), want (0,4)", x, y)
	}
}

func TestLayoutWrap(t *testing.T) {
	le := NewLayoutEngine()

	// Place a 12-wide panel
	le.Place(12, 7)
	// Place another 12-wide — should fit same row
	x, y := le.Place(12, 7)
	if x != 12 || y != 0 {
		t.Errorf("Place(12,7) = (%d,%d), want (12,0)", x, y)
	}

	// 13-wide won't fit — wraps
	x, y = le.Place(13, 5)
	if x != 0 || y != 7 {
		t.Errorf("Place(13,5) = (%d,%d), want (0,7)", x, y)
	}
}

func TestLayoutAddRow(t *testing.T) {
	le := NewLayoutEngine()

	// Place some panels
	le.Place(6, 4)
	le.Place(6, 4)

	// AddRow should advance past row
	y := le.AddRow()
	if y != 4 {
		t.Errorf("AddRow() = %d, want 4", y)
	}

	// Next panel should be at y=5 (row takes 1 unit)
	x, py := le.Place(12, 7)
	if x != 0 || py != 5 {
		t.Errorf("Place after row = (%d,%d), want (0,5)", x, py)
	}
}

func TestLayoutAddRowWhenEmpty(t *testing.T) {
	le := NewLayoutEngine()
	y := le.AddRow()
	if y != 0 {
		t.Errorf("AddRow() at start = %d, want 0", y)
	}
}

func TestLayoutFinishSection(t *testing.T) {
	le := NewLayoutEngine()
	le.Place(6, 4)
	le.Place(6, 8) // taller panel

	le.FinishSection()

	// Next panel should be at y=8
	x, y := le.Place(12, 5)
	if x != 0 || y != 8 {
		t.Errorf("Place after finish = (%d,%d), want (0,8)", x, y)
	}
}

func TestLayoutReset(t *testing.T) {
	le := NewLayoutEngine()
	le.Place(12, 7)
	le.Place(12, 7)
	le.Reset()

	x, y := le.Place(6, 4)
	if x != 0 || y != 0 {
		t.Errorf("Place after reset = (%d,%d), want (0,0)", x, y)
	}
}

func TestLayoutFullWidthPanel(t *testing.T) {
	le := NewLayoutEngine()
	x, y := le.Place(24, 8)
	if x != 0 || y != 0 {
		t.Errorf("Place(24,8) = (%d,%d), want (0,0)", x, y)
	}
	// Next panel must be on next row
	x, y = le.Place(6, 4)
	if x != 0 || y != 8 {
		t.Errorf("Place(6,4) after full = (%d,%d), want (0,8)", x, y)
	}
}
