package font

// Int26_6 is a signed 26.6 fixed-point number.
type Int26_6 int32

// Point26_6 is a point represented in 26.6 fixed-point coordinates.
type Point26_6 struct {
	X Int26_6
	Y Int26_6
}
