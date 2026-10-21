// Package text renders Iota Coder output using the fancy unicode styles that
// the bot uses for every user facing string.
package text

import "strings"

// Zero width non joiner keeps fancy glyphs from being merged by clients.
const (
	zwsp      = "\u200c"
	zwj       = "\u200d"
	separator = "\uAB59" // ꭙ
)

// smallLower maps lowercase latin letters onto the small-caps glyph set used
// for body copy.
var smallLower = map[rune]rune{
	'a': 'ɑ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'є', 'f': 'ꜰ',
	'g': 'ɢ', 'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ', 'k': 'ᴋ', 'l': 'ʟ',
	'm': 'ᴍ', 'n': 'η', 'o': 'σ', 'p': 'ᴘ', 'q': 'ǫ', 'r': 'ʀ',
	's': 's', 't': 'ᴛ', 'u': 'ᴜ', 'v': 'ᴠ', 'w': 'ω', 'x': 'x',
	'y': 'ʏ', 'z': 'ᴢ',
}

// smallUpper maps uppercase latin letters onto the small-caps glyph set.
var smallUpper = map[rune]rune{
	'A': 'ᴀ', 'B': 'ʙ', 'C': 'ᴄ', 'D': 'ᴅ', 'E': 'ᴇ', 'F': 'ꜰ',
	'G': 'ɢ', 'H': 'ʜ', 'I': 'ɪ', 'J': 'ᴊ', 'K': 'ᴋ', 'L': 'ʟ',
	'M': 'ᴍ', 'N': 'ɴ', 'O': 'ᴏ', 'P': 'ᴘ', 'Q': 'ǫ', 'R': 'ʀ',
	'S': 's', 'T': 'ᴛ', 'U': 'ᴜ', 'V': 'ᴠ', 'W': 'ᴡ', 'X': 'x',
	'Y': 'ʏ', 'Z': 'ᴢ',
}

// boldUpper maps uppercase latin letters onto mathematical bold sans-serif.
var boldUpper = map[rune]rune{
	'A': '𝐀', 'B': '𝐁', 'C': '𝐂', 'D': '𝐃', 'E': '𝐄', 'F': '𝐅',
	'G': '𝐆', 'H': '𝐇', 'I': '𝐈', 'J': '𝐉', 'K': '𝐊', 'L': '𝐋',
	'M': '𝐌', 'N': '𝐍', 'O': '𝐎', 'P': '𝐏', 'Q': '𝐐', 'R': '𝐑',
	'S': '𝐒', 'T': '𝐓', 'U': '𝐔', 'V': '𝐕', 'W': '𝐖', 'X': '𝐗',
	'Y': '𝐘', 'Z': '𝐙',
}

// titleLower maps lowercase latin letters onto the bold small-caps glyph set.
var titleLower = map[rune]rune{
	'a': 'ᴀ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'ᴇ', 'f': 'ꜰ',
	'g': 'ɢ', 'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ', 'k': 'ᴋ', 'l': 'ʟ',
	'm': 'ᴍ', 'n': 'ɴ', 'o': 'ᴏ', 'p': 'ᴘ', 'q': 'ǫ', 'r': 'ʀ',
	's': 'ꜱ', 't': 'ᴛ', 'u': 'ᴜ', 'v': 'ᴠ', 'w': 'ᴡ', 'x': 'x',
	'y': 'ʏ', 'z': 'ᴢ',
}

// boldLower differs from titleLower only for "k", which the brand line renders
// with a stroked capital glyph.
var boldLower = map[rune]rune{
	'a': 'ɑ', 'b': 'ʙ', 'c': 'ᴄ', 'd': 'ᴅ', 'e': 'є', 'f': 'ꜰ',
	'g': 'ɢ', 'h': 'ʜ', 'i': 'ɪ', 'j': 'ᴊ', 'k': 'ꝛ', 'l': 'ʟ',
	'm': 'ᴍ', 'n': 'η', 'o': 'σ', 'p': 'ᴘ', 'q': 'ǫ', 'r': 'ʀ',
	's': 's', 't': 'ᴛ', 'u': 'ᴜ', 'v': 'ᴠ', 'w': 'ω', 'x': 'x',
	'y': 'ʏ', 'z': 'ᴢ',
}

// Small renders text in the body small-caps style.
func Small(in string) string { return apply(in, smallLower, smallUpper, false) }

// Bold renders text in the brand header style.
func Bold(in string) string { return apply(in, boldLower, boldUpper, true) }

// Title renders text in the proper-noun style.
func Title(in string) string { return apply(in, titleLower, boldUpper, true) }

func apply(in string, lower, upper map[rune]rune, pad bool) string {
	var b strings.Builder
	b.Grow(len(in) * 2)
	for _, r := range in {
		switch {
		case r >= 'a' && r <= 'z':
			if g, ok := lower[r]; ok {
				b.WriteRune(g)
				continue
			}
		case r >= 'A' && r <= 'Z':
			if g, ok := upper[r]; ok {
				b.WriteRune(g)
				if pad {
					b.WriteString(zwsp)
				}
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Sep is the divider glyph used between brand words.
const Sep = separator

// Banner returns the Iota Coder brand line.
func Banner() string {
	return "˹" + Bold("Iota") + " " + Sep + " " + Bold("Coder") + "˼"
}

// Field renders a "label: value" pair using the label style.
func Field(label, value string) string {
	return Small(label) + ": " + value
}

// Status renders a check-marked status line.
func Status(label, value string) string {
	return SymCheck + " " + Small(label) + ": " + value
}

// Bullet renders a bullet list item.
func Bullet(text string) string { return SymBullet + " " + Small(text) }
