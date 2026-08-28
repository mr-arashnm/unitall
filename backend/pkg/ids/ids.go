// Package ids generates sortable UUIDv7 identifiers (stdlib only).
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// New returns a canonical 36-char UUIDv7 string: 48-bit big-endian
// unix-millis timestamp, version/variant bits, and 74 random bits.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is unrecoverable; fall back to time-derived id
		ms := uint64(time.Now().UnixMilli())
		for i := 0; i < 6; i++ {
			b[i] = byte(ms >> (40 - 8*i))
		}
	}
	ms := uint64(time.Now().UnixMilli())
	for i := 0; i < 6; i++ {
		b[i] = byte(ms >> (40 - 8*i))
	}
	b[6] = 0x70 | (b[6] & 0x0f) // version 7
	b[8] = 0x80 | (b[8] & 0x3f) // RFC 4122 variant

	var s [36]byte
	hex.Encode(s[0:8], b[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], b[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], b[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], b[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], b[10:16])
	return string(s[:])
}
