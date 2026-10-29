package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pfrayer/miam/internal/gmaps"
	"github.com/pfrayer/miam/internal/store"
	"github.com/pfrayer/miam/web"
)

// Config is read from a JSON file.
type Config struct {
	// Lists are in priority order (first list wins for a place's map icon).
	Lists        []ListConfig `json:"lists"`
	SyncInterval string       `json:"syncInterval"`
}

// ListConfig is one Google Maps list. In JSON it is either the bare share link
// or {"url": "...", "public": true}.
type ListConfig struct {
	URL string `json:"url"`
	// Public exposes the share link on the site so visitors can follow the
	// list in their own Google Maps (it shows the owner's name and avatar).
	Public bool `json:"public"`
	// Hidden keeps the list out of the default view (e.g. "to try" places).
	Hidden bool `json:"hidden"`
}

func (l *ListConfig) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &l.URL); err == nil {
		return nil
	}
	type plain ListConfig
	return json.Unmarshal(b, (*plain)(l))
}

func loadConfig(path string) (Config, time.Duration, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, 0, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, 0, fmt.Errorf("%s: %w", path, err)
	}
	if len(c.Lists) == 0 {
		return c, 0, fmt.Errorf("%s: no lists configured", path)
	}
	for i, l := range c.Lists {
		if u, err := url.Parse(l.URL); err != nil || u.Scheme != "https" || u.Host == "" {
			return c, 0, fmt.Errorf("%s: lists[%d]: %q is not an https URL", path, i, l.URL)
		}
	}
	every := time.Hour
	if c.SyncInterval != "" {
		if every, err = time.ParseDuration(c.SyncInterval); err != nil || every < time.Minute {
			return c, 0, fmt.Errorf("%s: syncInterval must be a duration >= 1m", path)
		}
	}
	return c, every, nil
}

// syncOnce fetches every list; any failure or empty list aborts the sync so the
// previous snapshot stays in place.
func syncOnce(ctx context.Context, client *gmaps.Client, cfg Config, st *store.Store) error {
	var lists []*gmaps.List
	var meta []store.ListMeta
	for _, lc := range cfg.Lists {
		id, err := client.ResolveListID(ctx, lc.URL)
		if err != nil {
			return err
		}
		l, err := client.FetchList(ctx, id)
		if err != nil {
			return err
		}
		if len(l.Places) == 0 {
			return fmt.Errorf("list %q (%s) came back empty", l.Name, id)
		}
		lists = append(lists, l)
		m := store.ListMeta{Hidden: lc.Hidden}
		if lc.Public {
			m.FollowURL = lc.URL
		}
		meta = append(meta, m)
	}
	snap := store.Build(lists, meta, time.Now().UTC())
	if err := st.Replace(snap); err != nil {
		return err
	}
	slog.Info("sync ok", "lists", len(snap.Categories), "places", len(snap.Places))
	return nil
}

// probe returns the process exit code for a healthcheck against the local server.
func probe(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func main() {
	cfgPath := flag.String("config", "config/config.json", "config file")
	dataPath := flag.String("data", "data/snapshot.json", "snapshot file")
	addr := flag.String("addr", ":8080", "listen address")
	healthcheck := flag.Bool("healthcheck", false, "probe the running server's /healthz and exit (for Docker, no shell in the image)")
	flag.Parse()

	if *healthcheck {
		os.Exit(probe(*addr))
	}

	cfg, every, err := loadConfig(*cfgPath)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(*dataPath)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &gmaps.Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			if err := syncOnce(ctx, client, cfg, st); err != nil {
				slog.Error("sync failed, keeping previous snapshot", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, web.FS, "index.html")
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /api/places", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st.Get())
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
