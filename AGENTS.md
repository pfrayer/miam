# Agent Instructions

miam publishes Google Maps saved lists as a read-only restaurant map. One Go
binary syncs the lists every hour (configurable) and serves a single embedded
page. Live at miam.pateenchroot.ovh.

## Commands

```sh
go vet ./...
go test ./...
gofmt -l .                 # must print nothing
docker build -t miam .
go run ./cmd/miam -config config/config.json -data data/snapshot.json   # http://localhost:8080
```

CI (`.github/workflows/ci.yml`) runs `go vet`, `go test` and `docker build` on
every PR and on `main`. Renovate keeps the Go image and the actions up to date.

## Layout

- `cmd/miam/main.go`: config loading (`config/config.json`, see
  `config/config.example.json`), the sync loop, the HTTP server: `GET /`
  (the page), `GET /api/places` (the snapshot as JSON), `GET /healthz`, and
  `-healthcheck` (probes `/healthz` for Docker: the image has no shell).
- `internal/gmaps`: resolves share links to list IDs and reads a list through
  the **undocumented** endpoint the Google Maps web UI uses
  (`/maps/preview/entitylist/getlist`). The response is a `)]}'`-prefixed,
  position-indexed JSON array: `ParseList` reads fields by index. It can break
  whenever Google changes it; `internal/gmaps/testdata/list.raw` is an
  anonymized captured response that `TestParseList` parses (the
  `gmaps-broke` skill covers the repair).
- `internal/store`: the last good snapshot, written atomically to
  `data/snapshot.json`. A sync that fails or returns an empty list never
  replaces it: the site keeps the previous version.
- `web/index.html`: the whole front end (HTML, CSS and JS in one file,
  embedded with `go:embed`): Leaflet and markercluster from cdnjs, the Figtree
  font from jsdelivr, map tiles from Stadia Maps.

## Rules

- **Privacy**: the owner's notes on places (`Place.Note`) are never
  serialized (`json:"-"`); keep it that way. `config/config.json` holds the
  private share links and is gitignored. Captured Google responses must be
  anonymized before they are committed (names, avatar, account IDs, notes,
  list IDs).
- **Production** runs from the `pfrayer/pateenchroot` repo (submodule, bumped
  by Renovate there): container `read_only`, no capabilities, non-root user
  65534, only `/data` writable, config mounted at `/config/config.json`. Do not
  write anywhere else at runtime.
- **CSP lives in pateenchroot** (`nginx/sites-available/miam.pateenchroot.ovh.conf`)
  and is checked in a browser by its CI (`tests/browser/csp-check.js`). When
  `index.html` starts loading something from a new origin (CDN, tiles, fonts,
  API), the CSP there must allow it, or the submodule bump fails CI there.
- Standard library only (no third-party Go modules); keep it that way unless
  asked.
