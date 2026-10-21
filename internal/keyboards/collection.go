package keyboards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/text"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// PrefixCollection marks the callback data of the menu of a collection.
const PrefixCollection = "pl:"

// The actions of the menu of a collection.
const (
	// CollectionActionAll sends every entry of the collection.
	CollectionActionAll = "a"
	// CollectionActionClose closes the menu.
	CollectionActionClose = "x"
)

// MaxCollectionLabel is the number of the characters a button of the menu
// carries.
//
// A button of a Telegram keyboard holds a short text, a long title is cut
// and the rest of it is dropped.
const MaxCollectionLabel = 34

// CollectionKeyboard builds the menu of a collection.
//
// The menu shows the entries of a page, every entry is a button, the page
// can be walked and the whole collection can be sent at once.
func CollectionKeyboard(collection *platforms.Collection, token string, page, pageSize int) *tg.InlineKeyboardMarkup {
	if collection == nil {
		return nil
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	total := len(collection.Items)
	pages := (total + pageSize - 1) / pageSize
	if pages <= 0 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}

	start := page * pageSize
	end := min(start+pageSize, total)

	rows := make([][]tg.InlineKeyboardButton, 0, pages+2)
	for index := start; index < end; index++ {
		item := collection.Items[index]
		label := CollectionLabel(index, item)
		rows = append(rows, Row(ColorButton(label, PrefixCollection+token+":"+strconv.Itoa(page)+":"+strconv.Itoa(index))))
	}

	navigation := []tg.InlineKeyboardButton{}
	if page > 0 {
		navigation = append(navigation, Button(text.Label(text.SymLeft, "previous"), PrefixCollection+token+":"+strconv.Itoa(page-1)+":p"))
	}
	if page+1 < pages {
		navigation = append(navigation, Button(text.Label(text.SymRight, "next"), PrefixCollection+token+":"+strconv.Itoa(page+1)+":p"))
	}
	if len(navigation) > 0 {
		rows = append(rows, navigation)
	}

	rows = append(rows, Row(
		Button(text.Label(text.SymDownload, "send all ("+strconv.Itoa(total)+")"), PrefixCollection+token+":0:"+CollectionActionAll),
		Button(text.Label(text.SymErase, "close"), PrefixCollection+token+":0:"+CollectionActionClose),
	))

	return Markup(rows...)
}

// CollectionLabel renders the label of an entry of a collection.
//
// The label carries the position of the entry, the name of the entry and the
// author of the entry when the entry has one.
func CollectionLabel(index int, item platforms.CollectionItem) string {
	name := strings.TrimSpace(item.Title)
	if name == "" {
		name = "item " + strconv.Itoa(index+1)
	}
	if len(name) > MaxCollectionLabel {
		name = strings.TrimSpace(name[:MaxCollectionLabel]) + "…"
	}
	label := strconv.Itoa(index+1) + ". " + name
	if item.Artist != "" && len(label)+len(item.Artist)+2 <= MaxCollectionLabel+8 {
		label += " · " + item.Artist
	}
	return text.Label(text.SymBullet, label)
}

// CollectionCallback builds the callback data of an entry of a collection.
func CollectionCallback(token string, page, index int) string {
	return fmt.Sprintf("%s%s:%d:%d", PrefixCollection, token, page, index)
}
