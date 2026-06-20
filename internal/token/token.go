package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloakia/opencloak/internal/types"
)

// keyLen is the number of bytes in a local key.
const keyLen = 32

// idBaseLen is the number of hex characters in the base token id (48 bits).
const idBaseLen = 12

// idExtLen is the number of additional hex characters appended on collision.
const idExtLen = 4

// Keyer holds a loaded local key and derives token ids.
type Keyer struct {
	key []byte
}

// NewKeyer loads the local key from path. If path is empty it defaults to
// ~/.opencloak/key. The file is created with mode 0600 and 32 random bytes if
// it does not exist.
func NewKeyer(path string) (*Keyer, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("token: resolve home dir: %w", err)
		}
		path = filepath.Join(home, ".opencloak", "key")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("token: read key file %s: %w", path, err)
		}
		// Generate a fresh key.
		data = make([]byte, keyLen)
		if _, err := rand.Read(data); err != nil {
			return nil, fmt.Errorf("token: generate key: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("token: create key dir: %w", err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, fmt.Errorf("token: write key file %s: %w", path, err)
		}
	}

	if len(data) < keyLen {
		return nil, fmt.Errorf("token: key file %s is too short (%d bytes, need %d)", path, len(data), keyLen)
	}
	return &Keyer{key: data[:keyLen]}, nil
}

// Derive returns the OpenCloak_<TYPE>_<id> token for (typ, value). The collision
// avoidance map records which normalized values already own each id within a
// single namespace; callers that need namespace isolation should pass a
// dedicated collision map. collisions maps id → normalized value; it is
// updated in-place so the caller can reuse it across multiple Derive calls.
func (k *Keyer) Derive(typ types.Type, value string, collisions map[string]string) string {
	norm := normalize(typ, value)
	id := k.hmacHex(norm)

	// Base id is first idBaseLen hex chars.
	candidate := id[:idBaseLen]

	// Walk the collision chain until we find an id whose owner is this value or
	// is unclaimed.
	for {
		if owner, exists := collisions[candidate]; !exists {
			// Unclaimed — register and return.
			collisions[candidate] = norm
			return format(typ, candidate)
		} else if owner == norm {
			// Same normalized value — deterministic, no collision.
			return format(typ, candidate)
		}
		// Genuine collision: different value owns this id. Extend.
		if len(candidate) >= len(id) {
			// Full HMAC hex exhausted (64 chars). This is astronomically
			// unlikely; panic is acceptable here.
			panic(fmt.Sprintf("token: hmac id space exhausted for type %s", typ))
		}
		next := len(candidate) + idExtLen
		if next > len(id) {
			next = len(id)
		}
		candidate = id[:next]
	}
}

// hmacHex returns the full hex encoding of HMAC-SHA256(key, data).
func (k *Keyer) hmacHex(data string) string {
	mac := hmac.New(sha256.New, k.key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// format assembles the OpenCloak_<TYPE>_<id> string.
func format(typ types.Type, id string) string {
	return Prefix + string(typ) + "_" + id
}

// normalize applies type-specific normalization to value before hashing.
// Rules:
//   - All types: TrimSpace.
//   - EMAIL: lowercase the domain part (after '@').
func normalize(typ types.Type, value string) string {
	v := strings.TrimSpace(value)
	if typ == types.TypeEmail {
		if at := strings.LastIndex(v, "@"); at >= 0 {
			v = v[:at+1] + strings.ToLower(v[at+1:])
		}
	}
	return v
}

// TokenPattern is the regexp source that matches any OpenCloak_… token, used by
// the restore scanner. The id part is at least idBaseLen hex chars.
const TokenPattern = `OpenCloak_[A-Z0-9]+_[0-9a-f]{12,}`
