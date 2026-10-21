package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// Defaults of the collection layer.
const (
	// DefaultCollectionMaxItems is the number of the entries of a collection
	// the bot expands.
	DefaultCollectionMaxItems = 50
	// DefaultCollectionPageSize is the number of the entries the menu of a
	// collection shows at once.
	DefaultCollectionPageSize = 10
)

// CollectionCache keeps the expanded collections of the last minutes in
// memory.
//
// A collection is expanded once and then offered in a menu, the menu holds
// the entries, so the entries must survive a few minutes. The entries expire
// on their own, a menu of an old collection is not answered any more.
type CollectionCache struct {
	mu      sync.Mutex
	entries map[string]collectionEntry
	ttl     time.Duration
	max     int
}

// collectionEntry is a cached collection with its expiry.
type collectionEntry struct {
	collection *platforms.Collection
	expires    time.Time
}

// NewCollectionCache builds a cache that keeps the collections for the given
// time. A ttl of zero turns the cache off.
func NewCollectionCache(ttl time.Duration, max int) *CollectionCache {
	if max <= 0 {
		max = 50
	}
	return &CollectionCache{
		entries: make(map[string]collectionEntry),
		ttl:     ttl,
		max:     max,
	}
}

// Get returns the cached collection of a token, nil when there is none.
func (c *CollectionCache) Get(token string) *platforms.Collection {
	if c == nil || c.ttl <= 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[token]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expires) {
		delete(c.entries, token)
		return nil
	}
	return entry.collection
}

// Put stores a collection under a token and returns the token.
func (c *CollectionCache) Put(collection *platforms.Collection) string {
	if c == nil || c.ttl <= 0 || collection == nil {
		return ""
	}

	token := CollectionToken(collection.Service, collection.ID)

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.max {
		c.pruneLocked()
	}
	if len(c.entries) >= c.max {
		for candidate := range c.entries {
			delete(c.entries, candidate)
			break
		}
	}
	c.entries[token] = collectionEntry{collection: collection, expires: time.Now().Add(c.ttl)}
	return token
}

// Len returns the number of cached collections.
func (c *CollectionCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// pruneLocked removes the expired entries of the cache.
func (c *CollectionCache) pruneLocked() {
	now := time.Now()
	for token, entry := range c.entries {
		if now.After(entry.expires) {
			delete(c.entries, token)
		}
	}
}

// CollectionToken builds the short token of a collection.
//
// The token is a part of the callback data of the menu, it must stay short.
func CollectionToken(service, id string) string {
	sum := sha256.Sum256([]byte(service + "|" + id))
	return hex.EncodeToString(sum[:])[:10]
}

// ResolveCollection expands a collection link into its entries.
//
// The engines of the bot are asked one after the other, the first one that
// answers delivers the entries. The order of the engines depends on the
// platform, the api of a platform is faster than the generic reader of
// yt-dlp.
func (r *Registry) ResolveCollection(ctx context.Context, ref platforms.CollectionRef, rawURL string, limit int) (*platforms.Collection, error) {
	if ref.ID == "" {
		return nil, fmt.Errorf("the collection has no id")
	}
	if limit <= 0 {
		limit = DefaultCollectionMaxItems
	}

	switch ref.Service {
	case platforms.ServiceYouTube, "youtube_music":
		collection, err := r.collectionFromIotaAPI(ctx, ref, limit)
		if err == nil {
			return collection, nil
		}
		if r.Log != nil {
			r.Log.Debug("iota api playlist failed: %v", err)
		}
		return r.collectionFromYTDLP(ctx, ref, rawURL, limit)

	case platforms.ServiceDeezer:
		if r.Deezer != nil {
			collection, err := r.Deezer.FetchPlaylist(ctx, ref.ID, limit)
			if err == nil {
				return collection, nil
			}
			if r.Log != nil {
				r.Log.Debug("deezer playlist failed: %v", err)
			}
		}
		return r.collectionFromIotaAPI(ctx, ref, limit)

	case platforms.ServiceSpotify:
		if r.Spotify != nil {
			collection, err := r.Spotify.FetchPlaylist(ctx, ref.ID, limit)
			if err == nil {
				return collection, nil
			}
			if r.Log != nil {
				r.Log.Debug("spotify playlist failed: %v", err)
			}
		}
		return nil, fmt.Errorf("the spotify playlist needs the credentials of an application")

	default:
		return r.collectionFromYTDLP(ctx, ref, rawURL, limit)
	}
}

// collectionFromYTDLP expands a collection with the generic reader of yt-dlp.
//
// The reader knows every platform yt-dlp supports, a playlist of YouTube, a
// set of SoundCloud and a list of Dailymotion are read the same way.
func (r *Registry) collectionFromYTDLP(ctx context.Context, ref platforms.CollectionRef, rawURL string, limit int) (*platforms.Collection, error) {
	if r.YTDLP == nil {
		return nil, fmt.Errorf("the reader of the collections is not ready")
	}
	info, err := r.YTDLP.ExtractPlaylist(ctx, platforms.CollectionReadURL(ref, rawURL))
	if err != nil {
		return nil, err
	}
	collection := platforms.CollectionFromPlaylist(ref.Service, info, limit)
	if collection == nil || len(collection.Items) == 0 {
		return nil, fmt.Errorf("the collection is empty")
	}
	collection.Kind = ref.Kind
	collection.ID = ref.ID
	collection.Title = platforms.PlaylistTitle(collection.Title)
	return collection, nil
}

// collectionFromIotaAPI expands a collection with the api of the bot.
//
// The api knows the playlists of YouTube and the playlists of Deezer, it
// keeps them in its own cache and answers them fast.
func (r *Registry) collectionFromIotaAPI(ctx context.Context, ref platforms.CollectionRef, limit int) (*platforms.Collection, error) {
	if !r.IotaAPI.Enabled() {
		return nil, fmt.Errorf("the iota api is not configured")
	}
	playlist, err := r.IotaAPI.Playlist(ctx, ref.ID, limit)
	if err != nil {
		return nil, err
	}
	if playlist == nil || len(playlist.Items) == 0 {
		return nil, fmt.Errorf("the collection is empty")
	}

	collection := &platforms.Collection{
		Service: ref.Service,
		Kind:    ref.Kind,
		ID:      ref.ID,
		Title:   playlist.Title,
		Total:   playlist.Total,
	}
	for index, item := range playlist.Items {
		link := CollectionItemLink(item.MediaID)
		if link == "" {
			continue
		}
		collection.Items = append(collection.Items, platforms.CollectionItem{
			Index:    index,
			Title:    item.Title,
			Artist:   item.Artist,
			URL:      link,
			Thumb:    item.Thumb,
			Duration: item.Duration,
		})
	}
	if collection.Total == 0 {
		collection.Total = len(collection.Items)
	}
	return collection, nil
}

// CollectionItemLink builds the link of an entry of a collection.
//
// The api names an entry by its media id, the id holds the platform of the
// entry and the id inside that platform.
func CollectionItemLink(mediaID string) string {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return ""
	}
	platform, id, found := strings.Cut(mediaID, ":")
	if !found {
		return ""
	}
	switch platform {
	case "yt", "youtube":
		return "https://www.youtube.com/watch?v=" + id
	case "dz", "dztrack":
		return "https://www.deezer.com/track/" + id
	case "sp", "spotify":
		return "https://open.spotify.com/track/" + id
	case "sc", "soundcloud":
		return mediaID
	default:
		return ""
	}
}
