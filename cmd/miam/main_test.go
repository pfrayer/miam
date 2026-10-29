package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigListForms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"lists": ["https://maps.app.goo.gl/a", {"url": "https://maps.app.goo.gl/b", "public": true}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []ListConfig{{URL: "https://maps.app.goo.gl/a"}, {URL: "https://maps.app.goo.gl/b", Public: true}}
	if len(c.Lists) != 2 || c.Lists[0] != want[0] || c.Lists[1] != want[1] {
		t.Errorf("lists = %+v", c.Lists)
	}
}

func TestLoadConfigRejectsNonHTTPS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"lists": [{"url": "javascript:alert(1)", "public": true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadConfig(path); err == nil {
		t.Error("expected an error for a non-https list URL")
	}
}
