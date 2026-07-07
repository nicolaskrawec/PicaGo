package compare

type Orientation int

const (
	OrientationVertical Orientation = iota
	OrientationHorizontal
)

type Slider struct {
	Position    float64
	Orientation Orientation
}

func (s *Slider) Clamp(min, max float64) {
	if s.Position < min {
		s.Position = min
	}
	if s.Position > max {
		s.Position = max
	}
}
