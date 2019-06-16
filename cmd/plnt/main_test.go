package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nnev/plnt"
)

func TestLoadConfig(t *testing.T) {
	const testConfig = `
cachedir = "/tmp/my/plnt-cache"

[feed.sur5r]
title = "sur5r’s blog"
url = "https://sur5r.net/"
`

	cfg, err := loadConfig([]byte(testConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.CacheDir, "/tmp/my/plnt-cache"; got != want {
		t.Errorf("Unexpected cfg.CacheDir: got %q, want %q", got, want)
	}
	want := map[string]plnt.FeedConfig{
		"sur5r": plnt.FeedConfig{
			ShortName: "sur5r",
			Title:     "sur5r’s blog",
			URL:       "https://sur5r.net/",
		},
	}
	if diff := cmp.Diff(want, cfg.Feeds); diff != "" {
		t.Errorf("Unexpected cfg.Feeds: diff (-want +got):\n%s", diff)
	}
}
