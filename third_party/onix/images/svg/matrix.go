package svg

import (
	"math"
	"strings"
)

type matrix struct {
	a float64
	b float64
	c float64
	d float64
	e float64
	f float64
}

func identityMatrix() matrix {
	return matrix{a: 1, d: 1}
}

func translateMatrix(x, y float64) matrix {
	return matrix{a: 1, d: 1, e: x, f: y}
}

func scaleMatrix(x, y float64) matrix {
	return matrix{a: x, d: y}
}

func rotateMatrix(rad float64) matrix {
	cos, sin := math.Cos(rad), math.Sin(rad)
	return matrix{a: cos, b: sin, c: -sin, d: cos}
}

func skewXMatrix(rad float64) matrix {
	return matrix{a: 1, c: math.Tan(rad), d: 1}
}

func skewYMatrix(rad float64) matrix {
	return matrix{a: 1, b: math.Tan(rad), d: 1}
}

func (m matrix) Mul(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m matrix) Transform(p point) point {
	return point{
		x: m.a*p.x + m.c*p.y + m.e,
		y: m.b*p.x + m.d*p.y + m.f,
	}
}

// Invert returns the inverse matrix. It reports false for a singular matrix,
// which paints nothing because no point can be mapped back.
func (m matrix) Invert() (matrix, bool) {
	determinant := m.a*m.d - m.b*m.c
	if determinant == 0 {
		return matrix{}, false
	}
	return matrix{
		a: m.d / determinant,
		b: -m.b / determinant,
		c: -m.c / determinant,
		d: m.a / determinant,
		e: (m.c*m.f - m.d*m.e) / determinant,
		f: (m.b*m.e - m.a*m.f) / determinant,
	}, true
}

func (m matrix) StrokeScale() float64 {
	x := math.Hypot(m.a, m.b)
	y := math.Hypot(m.c, m.d)
	if x == 0 {
		return y
	}
	if y == 0 {
		return x
	}
	return (x + y) * 0.5
}

func parseTransform(raw string) matrix {
	value := strings.TrimSpace(raw)
	if value == "" {
		return identityMatrix()
	}
	out := identityMatrix()
	for value != "" {
		value = strings.TrimLeft(value, " \t\r\n,")
		open := strings.IndexByte(value, '(')
		if open < 0 {
			break
		}
		name := strings.TrimSpace(value[:open])
		close := matchingParen(value, open)
		if close < 0 {
			break
		}
		args, err := parseNumberList(value[open+1 : close])
		if err == nil {
			out = out.Mul(transformFunction(name, args))
		}
		value = value[close+1:]
	}
	return out
}

func transformFunction(name string, args []float64) matrix {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "matrix":
		if len(args) >= 6 {
			return matrix{a: args[0], b: args[1], c: args[2], d: args[3], e: args[4], f: args[5]}
		}
	case "translate":
		if len(args) >= 1 {
			y := 0.0
			if len(args) >= 2 {
				y = args[1]
			}
			return translateMatrix(args[0], y)
		}
	case "scale":
		if len(args) >= 1 {
			y := args[0]
			if len(args) >= 2 {
				y = args[1]
			}
			return scaleMatrix(args[0], y)
		}
	case "rotate":
		if len(args) >= 1 {
			rad := args[0] * math.Pi / 180
			rot := rotateMatrix(rad)
			if len(args) >= 3 {
				return translateMatrix(args[1], args[2]).Mul(rot).Mul(translateMatrix(-args[1], -args[2]))
			}
			return rot
		}
	case "skewx":
		if len(args) >= 1 {
			return skewXMatrix(args[0] * math.Pi / 180)
		}
	case "skewy":
		if len(args) >= 1 {
			return skewYMatrix(args[0] * math.Pi / 180)
		}
	}
	return identityMatrix()
}

func matchingParen(value string, open int) int {
	depth := 0
	for i := open; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func viewportMatrix(viewBox Rect, width, height int, preserve PreserveAspectRatio) matrix {
	if viewBox.Width <= 0 || viewBox.Height <= 0 {
		return scaleMatrix(float64(width), float64(height))
	}
	sx := float64(width) / viewBox.Width
	sy := float64(height) / viewBox.Height
	if preserve.None {
		return scaleMatrix(sx, sy).Mul(translateMatrix(-viewBox.X, -viewBox.Y))
	}
	scale := math.Min(sx, sy)
	if preserve.Slice {
		scale = math.Max(sx, sy)
	}
	extraX := float64(width) - viewBox.Width*scale
	extraY := float64(height) - viewBox.Height*scale
	return translateMatrix(extraX*preserve.X, extraY*preserve.Y).Mul(scaleMatrix(scale, scale)).Mul(translateMatrix(-viewBox.X, -viewBox.Y))
}
