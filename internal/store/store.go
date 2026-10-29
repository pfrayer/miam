// Package store keeps the last valid snapshot of the synced lists, in memory
// and on disk, so a failing sync never empties the site.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pfrayer/miam/internal/gmaps"
)

// Category is one synced list.
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
	// FollowURL is the list's share link, set only for lists configured public.
	FollowURL string `json:"followUrl,omitempty"`
	// Hidden lists (e.g. "to try") are left out of the default view.
	Hidden bool `json:"hidden,omitempty"`
}

// ListMeta is the per-list site configuration passed to Build.
type ListMeta struct {
	FollowURL string
	Hidden    bool
}

// Place is a place merged across lists.
type Place struct {
	gmaps.Place
	MapsURL    string   `json:"mapsUrl"`
	Categories []string `json:"categories"` // category IDs, in config order
}

// Snapshot is what the API serves.
type Snapshot struct {
	SyncedAt   time.Time  `json:"syncedAt"`
	Categories []Category `json:"categories"`
	Places     []Place    `json:"places"`
}

// categoryID is a stable public ID for a list. The Google list ID itself must
// never be served: it is enough to open the list on Google Maps.
func categoryID(listID string) string {
	sum := sha256.Sum256([]byte(listID))
	return "c" + hex.EncodeToString(sum[:5])
}

// Build merges lists (in config order) into a snapshot, deduplicating places
// by Google feature ID. meta[i], when present, configures lists[i].
func Build(lists []*gmaps.List, meta []ListMeta, now time.Time) Snapshot {
	s := Snapshot{SyncedAt: now, Categories: []Category{}, Places: []Place{}}
	index := map[string]int{}
	for i, l := range lists {
		id := categoryID(l.ID)
		c := Category{ID: id, Name: l.Name, Emoji: l.Emoji}
		if i < len(meta) {
			c.FollowURL, c.Hidden = meta[i].FollowURL, meta[i].Hidden
		}
		s.Categories = append(s.Categories, c)
		for _, p := range l.Places {
			if i, ok := index[p.FeatureID]; ok {
				s.Places[i].Categories = append(s.Places[i].Categories, id)
				continue
			}
			index[p.FeatureID] = len(s.Places)
			s.Places = append(s.Places, Place{Place: p, MapsURL: p.MapsURL(), Categories: []string{id}})
		}
	}
	return s
}

// Store holds the current snapshot.
type Store struct {
	path string
	mu   sync.RWMutex
	cur  Snapshot
}

// Open loads the snapshot from path if it exists.
func Open(path string) (*Store, error) {
	s := &Store{path: path, cur: Snapshot{Categories: []Category{}, Places: []Place{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.cur); err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns the current snapshot.
func (s *Store) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Replace persists then swaps the snapshot.
func (s *Store) Replace(snap Snapshot) error {
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.mu.Lock()
	s.cur = snap
	s.mu.Unlock()
	return nil
}
