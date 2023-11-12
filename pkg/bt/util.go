package bt

// Max returns the larger of two ints
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the smaller of two ints
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ceil performs integer division and always rounds up
// It performs a + b - 1 / b since that is faster than
// than converting it to floats for math.Ceil
func Ceil(a, b int) int {
	return (a + b - 1) / b
}

// Floor performs integer division and always rounds down
func Floor(a, b int) int {
	return (a / b)
}
