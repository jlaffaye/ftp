package ftp

// A Scanner for fields delimited by one or more whitespace characters
type Scanner struct {
	bytes    []byte
	position int
}

// NewScanner creates a new Scanner
func NewScanner(str string) *Scanner {
	return &Scanner{
		bytes: []byte(str),
	}
}

// NextFields returns the next `count` fields
func (s *Scanner) NextFields(count int) []string {
	fields := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if field := s.Next(); field != "" {
			fields = append(fields, field)
		} else {
			break
		}
	}
	return fields
}

// Next returns the next field
func (s *Scanner) Next() string {
	sLen := len(s.bytes)

	// skip trailing whitespace
	for s.position < sLen {
		if s.bytes[s.position] != ' ' {
			break
		}
		s.position++
	}

	start := s.position

	// skip non-whitespace
	for s.position < sLen {
		if s.bytes[s.position] == ' ' {
			s.position++
			return string(s.bytes[start : s.position-1])
		}
		s.position++
	}

	return string(s.bytes[start:s.position])
}

// Remaining returns the remaining string
func (s *Scanner) Remaining() string {
	return string(s.bytes[s.position:len(s.bytes)])
}
