package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/renameio"
	"github.com/gorilla/feeds"
	"github.com/nnev/plnt"
	"github.com/nnev/plnt/internal/f2f"
)

// TODO: group posts by date
// TODO: group posts by feed
var htmlTmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"formatDay": func(t *time.Time) string {
		return t.Format("2006-01-02")
	},
	"formatPublished": func(t *time.Time) string {
		return t.Format("2006-01-02 15:04")
	},
	"formatLastUpdated": func(t *time.Time) string {
		return t.Format("2006-01-02 15:04")
	},
	"joinStrings": func(s []string) string {
		return strings.Join(s, ", ")
	},
}).Parse(`<!DOCTYPE html>
<head>
<title>{{ .Name }}</title>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<meta name="generator" content="plnt">
<link rel="stylesheet" href="planet.css" type="text/css">
<link rel="alternate" href="/atom.xml" title="{{ .Name }}" type="application/atom+xml">
</head>

<body>
<h1>{{ .Name }}</h1>

{{ range $idx, $item := .Items }}

<div class="daygroup">
<h2>{{ formatDay $item.PublishedParsed }}</h2>

<div class="channelgroup">
<h3><a href="{{ $item.FeedLink }}" title="{{ $item.FeedTitle }}">{{ $item.FeedTitle }}</a></h3>

<div class="entrygroup">
<h4><a href="{{ $item.Link }}">{{ $item.Title }}</a></h4>
<div class="entry">
<div class="content">
{{ if (eq $item.Content "") }}
{{ $item.Description }}
{{ else }}
{{ $item.Content }}
{{ end }}
</div>

<p class="date">
<a href="{{ $item.Link }}">{{ if $item.Author }}by {{ $item.Author.Name }}{{ end }}{{ if $item.PublishedParsed }} at {{ formatPublished $item.PublishedParsed }}{{ end }}
{{ $cat := joinStrings $item.Categories }}
{{ if $cat }}
under {{ $cat }}
{{ end }}
</a>
</p>
</div>
</div>

</div>
</div>
{{ end }}


<div class="sidebar">
<img src="images/logo.png" width="136" height="136" alt="">

<h2>Subscriptions</h2>
<ul>
{{ range $idx, $feed := .Feeds }}
<li>
<a href="{{ $feed.Link }}" title="subscribe"><img src="images/feed-icon-10x10.png" alt="(feed)"></a>
<a href="{{ $feed.Link }}" title="{{ $feed.Title }}">{{ $feed.Title }}</a>
</li>
{{ end }}
</ul>

<p>
<strong>Last updated:</strong><br>
{{ formatLastUpdated .LastUpdated }}<br>
<em>All times are UTC.</em><br>
<br>
Powered by:<br>
plnt
</p>

<p>
<h2>Planetarium:</h2>
<ul>
<li><a href="http://www.planetapache.org/">Planet Apache</a></li>
<li><a href="http://planet.freedesktop.org/">Planet freedesktop.org</a></li>
<li><a href="http://planet.gnome.org/">Planet GNOME</a></li>
<li><a href="http://planet.debian.net/">Planet Debian</a></li>
<li><a href="http://planet.fedoraproject.org/">Planet Fedora</a></li>
<li><a href="http://planets.sun.com/">Planet Sun</a></li>
<li><a href="http://www.planetplanet.org/">more...</a></li>
</ul>
</p>
</div>
</body>
</html>
`))

func writeHTML(fn string, feeds []*plnt.Feed, items []plnt.Item) error {
	var buf bytes.Buffer
	lastUpdated := time.Now().UTC()
	if err := htmlTmpl.Execute(&buf, struct {
		Items       []plnt.Item
		Feeds       []*plnt.Feed
		Name        string
		LastUpdated *time.Time
	}{
		Items:       items,
		Feeds:       feeds,
		Name:        "Planet NoName e.V.", // TODO
		LastUpdated: &lastUpdated,
	}); err != nil {
		return err
	}
	return renameio.WriteFile(fn, buf.Bytes(), 0644)
}

func writeFeed(fn string, items []plnt.Item) error {
	feed := feeds.Feed{
		Title: "merged",
		Link: &feeds.Link{
			Href: "https://todo.org/",
		},
	}
	for _, i := range items {
		feed.Add(f2f.GofeedToGorillaFeed(&i.Item))
	}
	atom, err := feed.ToAtom()
	if err != nil {
		return err
	}
	return renameio.WriteFile(fn, []byte(atom), 0644)
}

type config struct {
	CacheDir string                     `toml:"cachedir"`
	Feeds    map[string]plnt.FeedConfig `toml:"feed"`
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
	feedPath := flag.String("feed_path",
		"atom.xml",
		"path to write the output ATOM feed to")
	outputPath := flag.String("html_path",
		"index.html",
		"path to write the output HTML file to")

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
	var feedsCfg []plnt.FeedConfig
	for _, f := range cfg.Feeds {
		feedsCfg = append(feedsCfg, f)
	}
	aggr := &plnt.Aggregator{
		ForceFromCache: *forceFromCache,
		Feeds:          feedsCfg,
	}
	feeds, items, err := aggr.Fetch(ctx)
	if err != nil {
		log.Fatalf("Fetch: %v", err)
	}
	if len(items) > 30 {
		items = items[:30]
	}

	log.Printf("got %d items", len(items))
	if err := writeFeed(*feedPath, items); err != nil {
		log.Fatalf("writing aggregated feed: %v", err)
	}
	if err := writeHTML(*outputPath, feeds, items); err != nil {
		log.Fatalf("Writing HTML output: %v", err)
	}
}
