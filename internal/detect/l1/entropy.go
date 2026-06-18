package l1

import "math"

// shannonEntropy computes the Shannon entropy (bits per byte) of s.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	var freq [256]int
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	n := float64(len(s))
	var h float64
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / n
			h -= p * math.Log2(p)
		}
	}
	return h
}

// entropyThresholdHigh is the Shannon entropy threshold (bits/byte) above which
// a token is considered high-entropy. Base64url and similar encodings reach ~6 bits/byte.
const entropyThresholdHigh = 3.5

// contextKeywords are the words that, when appearing near a high-entropy
// string, promote it to a SECRET finding.
var contextKeywords = []string{
	"key", "secret", "token", "password", "apikey", "api_key",
	"credential", "credentials", "passwd", "pass", "auth",
	"access_key", "private_key", "secret_key",
}
