package l1

import "math"

// shannonEntropy computes the Shannon entropy (bits per symbol) of s.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, ch := range s {
		freq[ch]++
	}
	n := float64(len([]rune(s)))
	var h float64
	for _, count := range freq {
		p := float64(count) / n
		h -= p * math.Log2(p)
	}
	return h
}

// entropyThresholdHigh is the Shannon entropy threshold (bits/char) above which
// a token is considered high-entropy. Base64url and similar encodings reach ~6 bits/char.
const entropyThresholdHigh = 3.5

// contextKeywords are the words that, when appearing near a high-entropy
// string, promote it to a SECRET finding.
var contextKeywords = []string{
	"key", "secret", "token", "password", "apikey", "api_key",
	"credential", "credentials", "passwd", "pass", "auth",
	"access_key", "private_key", "secret_key",
}
