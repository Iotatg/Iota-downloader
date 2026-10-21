package text

import "strings"

// Symbols of the Iota Coder look.
//
// The bot does not use emoji, every symbol of the interface comes from this
// list, so the outputs and the buttons stay consistent on every client.
const (
	// SymBack marks the way back to the previous screen.
	SymBack = "☜"
	// SymForward marks the way forward to the next screen.
	SymForward = "☞"
	// SymLeft marks the left hand side of an action.
	SymLeft = "☚"
	// SymRight marks the right hand side of an action.
	SymRight = "☛"
	// SymUp marks the primary action of a screen.
	SymUp = "☝"
	// SymUpLeft marks a secondary action of a screen.
	SymUpLeft = "☟"
	// SymDelete marks a destructive action.
	SymDelete = "⌫"
	// SymErase marks a closing action.
	SymErase = "⌦"
	// SymStar marks the active entry of a selector.
	SymStar = "★"
	// SymStarOut marks an inactive entry of a selector.
	SymStarOut = "☆"
	// SymCheck marks a finished or successful action.
	SymCheck = "✔"
	// SymBar marks a section header.
	SymBar = "▌"
	// SymBullet marks a list entry.
	SymBullet = "▫"
	// SymRule marks a separator line.
	SymRule = "┈"
	// SymFilled marks the filled part of a progress bar.
	SymFilled = "▰"
	// SymEmpty marks the empty part of a progress bar.
	SymEmpty = "▱"
	// SymHex marks the extended list of platforms.
	SymHex = "⬡"
	// SymCross marks a failed action.
	SymCross = "✖"
	// SymArrow marks a step of a flow.
	SymArrow = "➜"
	// SymNote marks a group of platforms.
	SymNote = "◈"
	// SymSplit marks a file that was split into parts.
	SymSplit = "⧉"
	// SymDownload marks a download action.
	SymDownload = "↓"
	// SymMenu marks a menu action.
	SymMenu = "≡"
	// SymChart marks a statistics action.
	SymChart = "▤"
	// SymMusic marks an audio action.
	SymMusic = "♪"
	// SymLink marks a link action.
	SymLink = "↗"
	// SymInfo marks an information action.
	SymInfo = "ⓘ"
	// SymWait marks a running action.
	SymWait = "◷"
)

// Symbols lists every symbol of the bot, the order is the order of the
// interface: navigation, actions, states.
var Symbols = []string{
	SymBack, SymForward, SymLeft, SymRight, SymUp, SymUpLeft,
	SymDelete, SymErase, SymStar, SymStarOut, SymCheck, SymHex,
	SymCross, SymArrow, SymNote, SymSplit, SymDownload, SymMenu,
	SymChart, SymMusic, SymLink, SymInfo, SymWait,
}

// Label renders a symbol followed by a small caps label.
func Label(symbol, label string) string {
	if symbol == "" {
		return Small(label)
	}
	return symbol + " " + Small(label)
}

// Title renders a symbol followed by a fancy title.
func TitleWith(symbol, title string) string {
	if symbol == "" {
		return Title(title)
	}
	return symbol + " " + Title(title)
}

// Rule renders a separator line of the given width.
func Rule(width int) string {
	if width <= 0 {
		width = 26
	}
	return strings.Repeat(SymRule, width)
}

// Bar renders a textual progress bar.
func Bar(percent float64, width int) string {
	if width <= 0 {
		width = 10
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(width))
	return strings.Repeat(SymFilled, filled) + strings.Repeat(SymEmpty, width-filled)
}
