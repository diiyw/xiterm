package svg

import (
	"fmt"
	"strconv"
	"strings"
)

func parseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}

func parseNumberList(raw string) ([]float64, error) {
	scanner := numberScanner{s: raw}
	var values []float64
	for {
		scanner.skipSeparator()
		if scanner.done() {
			return values, nil
		}
		value, err := scanner.readNumber()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
}

type numberScanner struct {
	s string
	i int
}

func (s *numberScanner) done() bool {
	return s.i >= len(s.s)
}

func (s *numberScanner) skipSeparator() {
	for !s.done() {
		switch s.s[s.i] {
		case ' ', '\t', '\r', '\n', ',':
			s.i++
		default:
			return
		}
	}
}

func (s *numberScanner) hasNumber() bool {
	s.skipSeparator()
	if s.done() {
		return false
	}
	b := s.s[s.i]
	return b == '+' || b == '-' || b == '.' || ('0' <= b && b <= '9')
}

func (s *numberScanner) readNumber() (float64, error) {
	s.skipSeparator()
	start := s.i
	if !s.done() && (s.s[s.i] == '+' || s.s[s.i] == '-') {
		s.i++
	}
	digits := false
	for !s.done() && '0' <= s.s[s.i] && s.s[s.i] <= '9' {
		s.i++
		digits = true
	}
	if !s.done() && s.s[s.i] == '.' {
		s.i++
		for !s.done() && '0' <= s.s[s.i] && s.s[s.i] <= '9' {
			s.i++
			digits = true
		}
	}
	if !digits {
		return 0, fmt.Errorf("svg: expected number near %q", s.s[start:])
	}
	if !s.done() && (s.s[s.i] == 'e' || s.s[s.i] == 'E') {
		exp := s.i
		s.i++
		if !s.done() && (s.s[s.i] == '+' || s.s[s.i] == '-') {
			s.i++
		}
		expDigits := false
		for !s.done() && '0' <= s.s[s.i] && s.s[s.i] <= '9' {
			s.i++
			expDigits = true
		}
		if !expDigits {
			s.i = exp
		}
	}
	value, err := strconv.ParseFloat(s.s[start:s.i], 64)
	if err != nil {
		return 0, fmt.Errorf("svg: parse number %q: %w", s.s[start:s.i], err)
	}
	return value, nil
}
