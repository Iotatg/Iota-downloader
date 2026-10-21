package messages

import (
	"fmt"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/text"
)

// CollectionTitle renders the head of the menu of a collection.
//
// The menu names the collection, the platform it lives on and the number of
// the entries it holds.
func CollectionTitle(collection *platforms.Collection) string {
	if collection == nil {
		return ""
	}
	kind := platforms.CollectionKindLabel(collection.Kind)
	name := strings.TrimSpace(collection.Title)
	if name == "" {
		name = "untitled " + kind
	}
	lines := []string{
		text.Title(platforms.ServiceLabel(collection.Service) + " " + kind),
		text.Bullet(name),
	}
	switch {
	case len(collection.Items) == 0:
		lines = append(lines, text.Bullet("pick the entries you want."))
	case collection.Total > len(collection.Items):
		lines = append(lines, text.Bullet(fmt.Sprintf("%d of %d entries, pick the ones you want.", len(collection.Items), collection.Total)))
	default:
		lines = append(lines, text.Bullet(fmt.Sprintf("%d entries, pick the ones you want.", len(collection.Items))))
	}
	return strings.Join(lines, "\n")
}

// CollectionEntryLine renders the line of an entry of a collection.
func CollectionEntryLine(index int, item platforms.CollectionItem) string {
	name := strings.TrimSpace(item.Title)
	if name == "" {
		name = "entry " + fmt.Sprint(index+1)
	}
	line := fmt.Sprintf("%d. %s", index+1, name)
	if item.Artist != "" {
		line += " · " + item.Artist
	}
	return text.Bullet(line)
}

// CollectionEntryLabel renders the label of the button of an entry.
func CollectionEntryLabel(index int, item platforms.CollectionItem) string {
	name := strings.TrimSpace(item.Title)
	if name == "" {
		name = "entry " + fmt.Sprint(index+1)
	}
	if item.Artist != "" {
		name += " · " + item.Artist
	}
	return fmt.Sprintf("%d. %s", index+1, name)
}

// CollectionPage renders the foot of the menu of a collection.
func CollectionPage(page, pages int) string {
	if pages <= 1 {
		return ""
	}
	return text.Bullet(fmt.Sprintf("page %d/%d", page+1, pages))
}

// CollectionSending renders the status while a collection is sent.
func CollectionSending(current, total int) string {
	return item(text.SymWait, fmt.Sprintf("sending %d/%d of the collection...", current, total))
}

// CollectionSent closes a finished collection.
func CollectionSent(total int) string {
	return item(text.SymCheck, fmt.Sprintf("the collection is sent, %d entries delivered.", total))
}

// CollectionEmpty is sent when a collection holds no entry.
func CollectionEmpty() string {
	return item(text.SymCross, "this collection is empty.")
}

// CollectionNotSupported is sent when the platform of a collection has no
// reader of its own.
func CollectionNotSupported(service string) string {
	return item(text.SymCross, fmt.Sprintf("the %s playlists are not supported yet.", strings.ToLower(service))) + "\n" +
		item(text.SymBullet, "send a single track or video link of that platform, it is delivered right away.")
}
