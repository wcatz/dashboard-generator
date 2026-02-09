package generator

// IDGenerator produces auto-incrementing panel IDs.
type IDGenerator struct {
	id int
}

// NewIDGenerator creates a new ID generator.
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

// Reset resets the counter to 0.
func (g *IDGenerator) Reset() {
	g.id = 0
}

// Next returns the next panel ID.
func (g *IDGenerator) Next() int {
	g.id++
	return g.id
}
