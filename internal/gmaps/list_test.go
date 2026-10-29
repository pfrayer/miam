package gmaps

import (
	"os"
	"testing"
)

func TestParseList(t *testing.T) {
	body, err := os.ReadFile("testdata/list.raw")
	if err != nil {
		t.Fatal(err)
	}
	l, err := ParseList(body)
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "Resto Lyon" || l.Emoji != "🧑‍🍳" || l.ID != "AbCdEfGhIjKlMnOpQrStUvWxYz0123" {
		t.Errorf("header = %q %q %q", l.Name, l.Emoji, l.ID)
	}
	if len(l.Places) != 3 {
		t.Fatalf("places = %d, want 3", len(l.Places))
	}
	p := l.Places[0]
	if p.Name != "Brasserie Georges" || p.Address != "30 Cr de Verdun Perrache, 69002 Lyon" {
		t.Errorf("place = %+v", p)
	}
	if p.Lat < 45.74 || p.Lat > 45.75 || p.Lng < 4.82 || p.Lng > 4.83 {
		t.Errorf("coords = %v,%v", p.Lat, p.Lng)
	}
	if p.FeatureID != "0x47f4ea4b3d8af4f7:0x9fd71c9b0d71f8b2" || p.CID != "11517706024294414514" {
		t.Errorf("ids = %s %s", p.FeatureID, p.CID)
	}
}

func TestParseListRejectsGarbage(t *testing.T) {
	if _, err := ParseList([]byte("<html>")); err == nil {
		t.Error("expected error")
	}
}

func TestExtractListID(t *testing.T) {
	cases := map[string]string{
		"https://www.google.com/maps/@/data=!4m3!11m2!2sAbCdEfGhIjKlMnOpQrStUvWxYz0123!3e3?utm_source=x": "AbCdEfGhIjKlMnOpQrStUvWxYz0123",
		"https://www.google.com/maps/placelists/list/AbCdEfGhIjKlMnOpQrStUvWxYz0123":                     "AbCdEfGhIjKlMnOpQrStUvWxYz0123",
		"https://maps.app.goo.gl/FakeShortLink1":                                                         "",
	}
	for in, want := range cases {
		if got := extractListID(in); got != want {
			t.Errorf("extractListID(%q) = %q, want %q", in, got, want)
		}
	}
}
