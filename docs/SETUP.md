# Setting up miam

How to configure, run and work on your own miam. For what it does, see the [README](../README.md).

## How it works

```
Google Maps shared lists ──(hourly)──▶ miam (Go) ──▶ snapshot.json
                                          │
                                          └──▶ GET /           single page (Leaflet)
                                               GET /api/places merged places + categories
```

miam reads each list through the endpoint the Google Maps web app uses for shared lists. **It is not an official API**: Google can change it at any time. When that happens the site keeps serving the last snapshot while the parser gets fixed.

Map tiles come from [Stadia Maps](https://stadiamaps.com/) (free tier, authorized by domain, no key in the code). Google ratings are not embedded in the page: Google's terms only allow that on a Google map, so each place links to its Google Maps page instead.

## Configuration

Share each list from Google Maps (*Saved → list → Share*) and put the links in a JSON file. The order is the order of the filters, and the first list a place belongs to gives its marker emoji.

```json
{
  "lists": [
    { "url": "https://maps.app.goo.gl/AAAA", "public": true },
    "https://maps.app.goo.gl/BBBB",
    { "url": "https://maps.app.goo.gl/CCCC", "hidden": true }
  ],
  "syncInterval": "1h"
}
```

| Field | Meaning |
|---|---|
| `url` | The list's share link. A bare string is a private list. |
| `public` | Shows the share link on the site so visitors can follow the list in Google Maps. The list page shows your Google name and avatar. |
| `hidden` | Kept out of "all", the map and the counts until its filter is selected. |
| `syncInterval` | Go duration, at least `1m`. Defaults to `1h`. |

Private lists stay private: the site never serves their link or their Google list ID, nor the personal notes you add to places.

## Run it

With Docker:

```sh
docker build -t miam .
docker run -d --name miam -p 8080:8080 \
  -v "$PWD/config.json:/config/config.json:ro" \
  -v miam-data:/data \
  miam
```

Or from source (Go 1.22+):

```sh
cp config/config.example.json config/config.json   # then add your links
go run ./cmd/miam
```

Then open http://localhost:8080. In production, put it behind a TLS reverse proxy (geolocation needs HTTPS) and add your domain to your Stadia Maps account.

| Flag | Default | |
|---|---|---|
| `-config` | `config/config.json` | list configuration |
| `-data` | `data/snapshot.json` | last good sync |
| `-addr` | `:8080` | listen address |
| `-healthcheck` | | probe `/healthz` and exit (used by the image's `HEALTHCHECK`) |

## Development

```sh
go test ./...
```

CI (`.github/workflows/ci.yml`) runs `go vet`, `go test` and a Docker build on every pull request and on `main`. Dependencies are kept up to date by Renovate.

The front end is a single file, [`web/index.html`](../web/index.html), embedded into the binary: vanilla JS, [Leaflet](https://leafletjs.com/) and [Leaflet.markercluster](https://github.com/Leaflet/Leaflet.markercluster) from a CDN, no build step.
