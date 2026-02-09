package generator

// LayoutEngine implements the 24-unit grid auto-layout algorithm.
type LayoutEngine struct {
	GridWidth int
	cursorX   int
	cursorY   int
	rowHeight int
}

// NewLayoutEngine creates a new layout engine with the standard 24-unit grid.
func NewLayoutEngine() *LayoutEngine {
	return &LayoutEngine{GridWidth: 24}
}

// Reset resets the layout state for a new dashboard.
func (le *LayoutEngine) Reset() {
	le.cursorX = 0
	le.cursorY = 0
	le.rowHeight = 0
}

// AddRow forces a new line and returns the y position for the row panel.
func (le *LayoutEngine) AddRow() int {
	if le.cursorX > 0 {
		le.cursorY += le.rowHeight
		le.cursorX = 0
		le.rowHeight = 0
	}
	y := le.cursorY
	le.cursorY++
	le.cursorX = 0
	le.rowHeight = 0
	return y
}

// Place positions a panel and returns (x, y) coordinates.
func (le *LayoutEngine) Place(width, height int) (int, int) {
	if le.cursorX+width > le.GridWidth {
		le.cursorY += le.rowHeight
		le.cursorX = 0
		le.rowHeight = 0
	}
	x := le.cursorX
	y := le.cursorY
	le.cursorX += width
	if height > le.rowHeight {
		le.rowHeight = height
	}
	return x, y
}

// FinishSection advances past the tallest panel in the current row.
func (le *LayoutEngine) FinishSection() {
	if le.cursorX > 0 {
		le.cursorY += le.rowHeight
		le.cursorX = 0
		le.rowHeight = 0
	}
}
