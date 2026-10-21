package text

import (
	"hash/fnv"
	"strings"
)

// AccentPalette holds the colourful glyphs used to decorate the buttons and the
// messages of the bot. Beside the native button colours of the Bot API, the
// leading glyph of a button keeps two buttons apart even on clients that
// ignore the colour of a button.
//
// The palette holds geometric shapes only, the bot does not use emoji.
var AccentPalette = []string{
	"◈", "◆", "◇", "◉", "◐", "◑", "◒", "◓",
	"▣", "▤", "▥", "▦", "▧", "▨", "▩", "◊",
}

// Accent returns a stable colourful glyph for a seed.
//
// The same seed always produces the same glyph, so a button keeps its colour
// between messages while different buttons look different from each other.
func Accent(seed string) string {
	if len(AccentPalette) == 0 {
		return ""
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	return AccentPalette[int(hash.Sum32()%uint32(len(AccentPalette)))]
}

// AccentFor returns the colourful glyph of a seed followed by the given label.
func AccentFor(seed, label string) string {
	glyph := Accent(seed)
	if glyph == "" {
		return label
	}
	return glyph + " " + label
}

// AccentLabel puts a colourful glyph in front of a label.
//
// A label that already carries one of the symbols of the bot keeps it, the
// accent is only added to a plain label. A leading selection marker keeps its
// place so the active entry of a selector stays readable.
func AccentLabel(seed, label string) string {
	glyph := Accent(seed)
	if glyph == "" {
		return label
	}
	if HasSymbol(label) {
		return label
	}
	trimmed := strings.TrimSpace(label)
	if strings.HasPrefix(trimmed, "• ") {
		return "• " + glyph + " " + strings.TrimPrefix(trimmed, "• ")
	}
	return glyph + " " + StripAccent(label)
}

// HasSymbol reports whether a label starts with one of the symbols of the bot.
func HasSymbol(label string) bool {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return false
	}
	for _, symbol := range Symbols {
		if strings.HasPrefix(trimmed, symbol) {
			return true
		}
	}
	return false
}

// StripAccent removes a leading glyph and its trailing space from a label.
func StripAccent(label string) string {
	runes := []rune(label)
	index := 0
	for index < len(runes) {
		size := emojiSize(runes[index])
		if size == 0 {
			break
		}
		index += size
	}
	for index < len(runes) && (runes[index] == ' ' || runes[index] == '\u200d' || runes[index] == '\ufe0f') {
		index++
	}
	return string(runes[index:])
}

// emojiSize returns the length of the emoji starting at the given rune.
func emojiSize(runeValue rune) int {
	switch {
	case runeValue >= 0x1F300 && runeValue <= 0x1FAFF:
		return 2
	case runeValue >= 0x1F000 && runeValue <= 0x1F2FF:
		return 2
	case runeValue >= 0x2600 && runeValue <= 0x27BF:
		return 2
	case runeValue >= 0x2B00 && runeValue <= 0x2BFF:
		return 2
	case runeValue >= 0x1F1E6 && runeValue <= 0x1F1FF:
		return 2
	case runeValue == 0x203C || runeValue == 0x2049 || runeValue == 0x2122 || runeValue == 0x2139:
		return 1
	case runeValue == 0x2194 || runeValue == 0x2195 || runeValue == 0x2199:
		return 1
	case runeValue == 0x2714 || runeValue == 0x2716 || runeValue == 0x2718:
		return 1
	case runeValue == 0x2757 || runeValue == 0x27A1:
		return 1
	default:
		return 0
	}
}
