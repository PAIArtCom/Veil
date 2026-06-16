package l1

import "strings"

// ibanValid validates an IBAN using the ISO 13616 mod-97 check.
func ibanValid(s string) bool {
	clean := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", ""))
	if len(clean) < 15 || len(clean) > 34 {
		return false
	}
	for _, r := range clean {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}

	rearranged := clean[4:] + clean[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			n := int(r-'A') + 10
			remainder = (remainder*10 + n/10) % 97
			remainder = (remainder*10 + n%10) % 97
		default:
			return false
		}
	}
	return remainder == 1
}
