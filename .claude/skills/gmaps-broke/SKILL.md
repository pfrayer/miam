---
name: gmaps-broke
description: Diagnose and repair the Google Maps list sync when it stops working (sync failed in the logs, empty or stale map, getlist status errors, parse errors). Use when the user says the sync, the lists or miam are broken or out of date.
argument-hint: [error message or symptom]
---

# Repair the Google Maps sync: $ARGUMENTS

miam reads lists through an undocumented Google endpoint (AGENTS.md,
`internal/gmaps`). When Google changes it, the sync fails and the site keeps
the last good snapshot: there is no outage, take the time to do it right.

## 1. Find which step fails

Production logs are in the pateenchroot stack (`docker compose logs miam` on
the VPS: ask the user, it is production). Locally, with the user's real
`config/config.json`:

```sh
go run ./cmd/miam -config config/config.json -data "$(mktemp -d)/snapshot.json" -addr :18080
```

and read the `sync failed` line:

- **resolving the share link** (`ResolveListID`): redirects or the URL shape
  changed → `extractListID` regexes (`dataListID`, `pathListID`).
- **`getlist ... status 4xx/5xx`**: the request changed (URL, `pb` parameter,
  headers, user agent) → `FetchList`. Compare with the request the Google Maps
  web UI sends for the same list (browser dev tools, `getlist`).
- **`unexpected getlist response prefix` / `decode getlist`**: the envelope
  changed → `ParseList` prefix handling.
- **`came back empty`** or wrong fields: the positions in the array moved →
  the `at(...)` indexes in `ParseList`.

## 2. Capture a fresh response

Fetch the same URL `FetchList` builds for one real list ID (from the user's
config, resolved by `ResolveListID`) and save the raw body. Then **anonymize
it before it goes anywhere near git**: replace the owner name, avatar URL,
account ID, list IDs, and every note on places (`Note`, index 3 of each item)
with neutral values, keeping the structure, types and array lengths intact.
Keep 3 places, as in the current fixture, with realistic coordinates and
addresses (public restaurants are fine). Ask the user to check the anonymized
file before committing it.

## 3. Test first

Replace `internal/gmaps/testdata/list.raw` with the anonymized capture and
update `TestParseList` expectations to its values. Run `go test ./internal/gmaps/`:
it must fail the same way production does. Then fix `ParseList` / `FetchList`
until it passes, keeping the existing guarantees: places without a name or
IDs are skipped, notes are never serialized, an empty list aborts the sync.

## 4. Verify for real

Run the local server against the real config again: the log must show
`sync ok` with the expected list and place counts, and
`curl -s localhost:18080/api/places` must return them (no notes in it).

Then the usual: `gofmt -l .` empty, `go vet ./...`, `go test ./...`, a PR with
`fix(gmaps): …`. Production picks it up when Renovate bumps the submodule in
pateenchroot.
