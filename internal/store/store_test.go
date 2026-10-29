package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pfrayer/miam/internal/gmaps"
)

func TestBuildMergesByFeatureID(t *testing.T) {
	a := gmaps.Place{FeatureID: "0x1:0x1", CID: "1", Name: "A"}
	b := gmaps.Place{FeatureID: "0x2:0x2", CID: "2", Name: "B"}
	snap := Build([]*gmaps.List{
		{ID: "asiat", Places: []gmaps.Place{a, b}},
		{ID: "pascher", Places: []gmaps.Place{b}},
	}, nil, time.Unix(0, 0))
	if len(snap.Places) != 2 || len(snap.Categories) != 2 {
		t.Fatalf("got %d places, %d categories", len(snap.Places), len(snap.Categories))
	}
	if got := snap.Places[1].Categories; len(got) != 2 || got[0] != categoryID("asiat") || got[1] != categoryID("pascher") {
		t.Errorf("B categories = %v", got)
	}
	if snap.Places[0].MapsURL != "https://maps.google.com/?cid=1" {
		t.Errorf("mapsUrl = %s", snap.Places[0].MapsURL)
	}
}

func TestBuildHidesGoogleListIDAndNotes(t *testing.T) {
	const listID = "AbCdEfGhIjKlMnOpQrStUvWxYz0123"
	snap := Build([]*gmaps.List{{ID: listID, Name: "Resto Lyon",
		Places: []gmaps.Place{{FeatureID: "0x1:0x1", Name: "A", Note: "secret"}}}}, nil, time.Unix(0, 0))
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{listID, "secret"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("snapshot JSON leaks %q: %s", leak, b)
		}
	}
}

func TestBuildFollowURLOnlyForPublicLists(t *testing.T) {
	snap := Build([]*gmaps.List{{ID: "pub"}, {ID: "priv"}},
		[]ListMeta{{FollowURL: "https://maps.app.goo.gl/abc"}, {Hidden: true}}, time.Unix(0, 0))
	if got := snap.Categories[0].FollowURL; got != "https://maps.app.goo.gl/abc" {
		t.Errorf("public followUrl = %q", got)
	}
	b, _ := json.Marshal(snap.Categories[1])
	if strings.Contains(string(b), "followUrl") {
		t.Errorf("private list exposes followUrl: %s", b)
	}
	if snap.Categories[0].Hidden || !snap.Categories[1].Hidden {
		t.Errorf("hidden flags = %v, %v", snap.Categories[0].Hidden, snap.Categories[1].Hidden)
	}
}

func TestStorePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "snapshot.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := Build([]*gmaps.List{{ID: "l", Places: []gmaps.Place{{FeatureID: "0x1:0x1", Name: "A"}}}}, nil, time.Unix(42, 0))
	if err := s.Replace(snap); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Get(); len(got.Places) != 1 || got.Places[0].Name != "A" || !got.SyncedAt.Equal(time.Unix(42, 0)) {
		t.Errorf("reloaded = %+v", got)
	}
}
