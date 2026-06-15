package l1

// luhnValid returns true if s (digits only) passes the Luhn check.
// It returns false if s is empty or contains non-digit characters.
func luhnValid(s string) bool {
	if len(s) < 2 {
		return false
	}
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return false
		}
		n := int(ch - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}
