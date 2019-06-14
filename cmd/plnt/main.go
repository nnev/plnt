package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"text/template"

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

func main() {
	forceFromCache := flag.Bool("force_from_cache",
		false,
		"force loading feeds from cache (prevents any network access). useful for a rapid feedback cycle during development")
	flag.Parse()
	log.Printf("ohai")
	ctx := context.Background()
	// TODO: load configuration from toml file? what config does planet use?
	aggr := &plnt.Aggregator{
		ForceFromCache: *forceFromCache,
		Feeds: []plnt.Feed{
			{
				ShortName: "sur5r",
				Title:     "sur5r/blog",
				// ATOM feed does not work; chokes on
				// “<479BAFEE.4040500@secure-endpoints.com>” in
				// https://blogs.noname-ev.de/sur5r/index.php?/archives/11-guid.html
				URL: "https://blogs.noname-ev.de/sur5r/index.php?/feeds/index.rss2",
			},

			{
				ShortName: "cherti",
				Title:     "Insanity Industries",
				URL:       "https://insanity.industries/index.xml",
			},

			{
				ShortName: "secure",
				Title:     "sECuREs Website",
				URL:       "https://michael.stapelberg.ch/feed.xml",
			},
		},
	}
	items, err := aggr.Fetch(ctx)
	if err != nil {
		log.Fatalf("Fetch: %v", err)
	}
	// TODO: need to canonicalize all relative URLs, i.e. parse the HTML?

	log.Printf("got %d items", len(items))
	if err := writeFeed("/tmp/feed.atom", items); err != nil {
		log.Fatalf("writing aggregated feed: %v", err)
	}
	if err := writeHTML("/tmp/out.html", items); err != nil {
		log.Fatalf("Writing HTML output: %v", err)
	}
}
