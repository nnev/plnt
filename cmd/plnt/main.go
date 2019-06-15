package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/google/renameio"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
	"github.com/nnev/plnt"
	"github.com/nnev/plnt/internal/f2f"
)

var htmlTmpl = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
  <head>
    <title>plnt</title>
  </head>
  <body>
    <ul>
    {{ range $idx, $item := .Items }}
      <li>item {{ $idx }}, title {{ $item.Title }}</li>
    {{ end }}
    </ul>
  </body>
</html>
`))

func writeHTML(fn string, items []*gofeed.Item) error {
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, struct {
		Items []*gofeed.Item
	}{
		Items: items,
	}); err != nil {
		return err
	}
	return renameio.WriteFile(fn, buf.Bytes(), 0644)
}

func writeFeed(fn string, items []*gofeed.Item) error {
	feed := feeds.Feed{
		Title: "merged",
		Link: &feeds.Link{
			Href: "https://todo.org/",
		},
	}
	for _, i := range items {
		feed.Add(f2f.GofeedToGorillaFeed(i))
	}
	atom, err := feed.ToAtom()
	if err != nil {
		return err
	}
	return renameio.WriteFile(fn, []byte(atom), 0644)
}

type config struct {
	CacheDir string               `toml:"cachedir"`
	Feeds    map[string]plnt.Feed `toml:"feed"`
}

func loadConfig(b []byte) (*config, error) {
	var cfg config
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	for shortname, feed := range cfg.Feeds {
		feed.ShortName = shortname
		cfg.Feeds[shortname] = feed
	}
	return &cfg, nil
}

func main() {
	forceFromCache := flag.Bool("force_from_cache",
		false,
		"force loading feeds from cache (prevents any network access). useful for a rapid feedback cycle during development")
	configPath := flag.String("config",
		"/etc/plnt.toml",
		"path to the configuration file")
	flag.Parse()
	ctx := context.Background()

	b, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := loadConfig(b)
	if err != nil {
		log.Fatal(err)
	}
	var feeds []plnt.Feed
	for _, f := range cfg.Feeds {
		feeds = append(feeds, f)
	}
	aggr := &plnt.Aggregator{
		ForceFromCache: *forceFromCache,
		Feeds:          feeds,
	}
	items, err := aggr.Fetch(ctx)
	if err != nil {
		log.Fatalf("Fetch: %v", err)
	}

	log.Printf("got %d items", len(items))
	if err := writeFeed("/tmp/feed.atom", items); err != nil {
		log.Fatalf("writing aggregated feed: %v", err)
	}
	if err := writeHTML("/tmp/out.html", items); err != nil {
		log.Fatalf("Writing HTML output: %v", err)
	}
}
