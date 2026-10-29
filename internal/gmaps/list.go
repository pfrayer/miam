// Package gmaps reads shared Google Maps lists through the undocumented
// endpoint used by the Google Maps web UI. It can break without notice.
package gmaps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

// List is a Google Maps saved list.
type List struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Emoji  string  `json:"emoji"`
	Places []Place `json:"places"`
}

// Place is one entry of a list.
type Place struct {
	// FeatureID is Google's internal "0x..:0x.." identifier, stable across lists.
	FeatureID string  `json:"featureId"`
	CID       string  `json:"cid"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	// Note is the owner's private note on the place: never serialized.
	Note string `json:"-"`
}

// MapsURL opens the place in Google Maps.
func (p Place) MapsURL() string { return "https://maps.google.com/?cid=" + p.CID }

var (
	dataListID = regexp.MustCompile(`!2s([A-Za-z0-9_-]{10,})!3e3`)
	pathListID = regexp.MustCompile(`/placelists/list/([A-Za-z0-9_-]{10,})`)
)

// Client fetches lists.
type Client struct {
	HTTP *http.Client
}

// ResolveListID turns a share link (maps.app.goo.gl/...) or a full Google Maps
// list URL into a list ID, following redirects if needed.
func (c *Client) ResolveListID(ctx context.Context, shareURL string) (string, error) {
	if id := extractListID(shareURL); id != "" {
		return id, nil
	}
	u, err := url.Parse(shareURL)
	if err != nil || u.Scheme != "https" {
		return "", fmt.Errorf("invalid list URL %q", shareURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, shareURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	// Do not follow redirects: the list ID is in the first Location header.
	hc := *c.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if id := extractListID(resp.Header.Get("Location")); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("no list ID found behind %q (status %d)", shareURL, resp.StatusCode)
}

func extractListID(s string) string {
	for _, re := range []*regexp.Regexp{dataListID, pathListID} {
		if m := re.FindStringSubmatch(s); m != nil {
			return m[1]
		}
	}
	return ""
}

// FetchList downloads and parses a list by ID.
func (c *Client) FetchList(ctx context.Context, id string) (*List, error) {
	u := "https://www.google.com/maps/preview/entitylist/getlist?authuser=0&hl=fr&gl=fr&pb=" +
		"!1m4!1s" + url.QueryEscape(id) + "!2e1!3m1!1e1!2e2!3e2!4i500!16b1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getlist %s: status %d", id, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return ParseList(body)
}

// ParseList parses a getlist response body.
func ParseList(body []byte) (*List, error) {
	_, payload, found := strings.Cut(string(body), "\n")
	if !found || !strings.HasPrefix(string(body), ")]}'") {
		return nil, errors.New("unexpected getlist response prefix")
	}
	var root []any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return nil, fmt.Errorf("decode getlist: %w", err)
	}
	head := arr(at(root, 0))
	if head == nil {
		return nil, errors.New("getlist: missing list header")
	}
	l := &List{
		ID:    str(at(arr(at(head, 0)), 0)),
		Name:  str(at(head, 4)),
		Emoji: str(at(head, 17)),
	}
	for _, raw := range arr(at(head, 8)) {
		item := arr(raw)
		loc := arr(at(item, 1))
		coords := arr(at(loc, 5))
		ids := arr(at(loc, 6))
		p := Place{
			Name:    str(at(item, 2)),
			Note:    str(at(item, 3)),
			Address: str(at(loc, 4)),
			Lat:     num(at(coords, 2)),
			Lng:     num(at(coords, 3)),
		}
		hi, errHi := strconv.ParseInt(str(at(ids, 0)), 10, 64)
		lo, errLo := strconv.ParseInt(str(at(ids, 1)), 10, 64)
		if p.Name == "" || errHi != nil || errLo != nil {
			continue // skip entries we cannot identify
		}
		p.FeatureID = fmt.Sprintf("0x%x:0x%x", uint64(hi), uint64(lo))
		p.CID = strconv.FormatUint(uint64(lo), 10)
		l.Places = append(l.Places, p)
	}
	return l, nil
}

func at(a []any, i int) any {
	if i < len(a) {
		return a[i]
	}
	return nil
}

func arr(v any) []any   { a, _ := v.([]any); return a }
func str(v any) string  { s, _ := v.(string); return s }
func num(v any) float64 { f, _ := v.(float64); return f }
