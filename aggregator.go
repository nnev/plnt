package plnt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/renameio"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/sync/errgroup"
)

type Feed struct {
	ShortName string // used in the state directory name, e.g. “sur5r-blog”
	Title     string `toml:"title"` // human-readable, e.g. “sur5r’s Hardware Blog”
	URL       string `toml:"url"`
}

func (f *Feed) cachePath() string {
	// TODO: make cache dir overridable via config
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "/var/cache"
	}
	return filepath.Join(cacheDir, "plnt", f.ShortName+".json")
}

type Aggregator struct {
	Feeds []Feed

	ForceFromCache bool // force loading all files from cache for rapid development
}

func makeAbsolute(content, baseURL string) (string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(content), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return "", err
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for idx, a := range n.Attr {
				if a.Key != "src" {
					continue
				}
				if strings.Contains(a.Val, "://") {
					continue // already absolute
				}
				// TODO(later): is there a more elegant way to join baseURL with a.Val in terms of path?
				a.Val = strings.TrimSuffix(baseURL, "/") + a.Val
				n.Attr[idx] = a
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	var out bytes.Buffer
	for _, n := range nodes {
		f(n)
		if err := html.Render(&out, n); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func from(r io.Reader, feed *Feed) (*gofeed.Feed, error) {
	parser := gofeed.NewParser()
	f, err := parser.Parse(r)
	if err != nil {
		return nil, err
	}

	items := make([]*gofeed.Item, 0, len(f.Items))
	for _, i := range f.Items {
		if i.PublishedParsed == nil {
			// fall back to the updated date, if any
			i.Published = i.Updated
			i.PublishedParsed = i.UpdatedParsed
			if i.PublishedParsed == nil {
				log.Printf("[%s] dropping post %q: neither published date nor updated date set", feed.ShortName, i.Title)
				continue
			}
		}
		i.Title = fmt.Sprintf("[%s] %s", feed.Title, i.Title)
		var err error
		i.Content, err = makeAbsolute(i.Content, feed.URL)
		if err != nil {
			return nil, fmt.Errorf("makeAbsolute(%s): %v", feed.ShortName, err)
		}
		items = append(items, i)
	}
	f.Items = items

	return f, nil
}

func fromCache(feed *Feed) (*gofeed.Feed, error) {
	b, err := ioutil.ReadFile(feed.cachePath())
	if err != nil {
		return nil, err
	}
	var f gofeed.Feed
	return &f, json.Unmarshal(b, &f)
}

func (a *Aggregator) fetchFeed(ctx context.Context, feed *Feed) (*gofeed.Feed, error) {
	if a.ForceFromCache {
		return fromCache(feed)
	}
	var modTime time.Time
	if st, err := os.Stat(feed.cachePath()); err == nil {
		modTime = st.ModTime()
	}
	log.Printf("[%s] fetching %s", feed.ShortName, feed.URL)
	req, err := http.NewRequest("GET", feed.URL, nil)
	if err != nil {
		return nil, err
	}
	if !modTime.IsZero() {
		req.Header.Set("If-Modified-Since", modTime.Format(http.TimeFormat))
	}
	// TODO: set user agent
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[%s] falling back to cache due to fetch error: %v", feed.ShortName, err)
		return fromCache(feed)
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		if resp.StatusCode == http.StatusNotModified {
			return fromCache(feed)
		}
		log.Printf("[%s] falling back to cache due to fetch error: unexpected status code: got %v, want %v", feed.ShortName, got, want)
		return fromCache(feed)
	}
	f, err := from(resp.Body, feed)
	if err != nil {
		log.Printf("[%s] falling back to cache due to fetch error: %v", feed.ShortName, err)
		return fromCache(feed)
	}
	// Exhaust the reader to make HTTP connection pooling/keepalive work:
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	fn := feed.cachePath()
	if err := os.MkdirAll(filepath.Dir(fn), 0755); err != nil {
		return nil, err
	}
	if err := renameio.WriteFile(fn, b, 0644); err != nil {
		return nil, err
	}

	return f, nil
}

func (a *Aggregator) Fetch(ctx context.Context) ([]*gofeed.Item, error) {
	eg, ctx := errgroup.WithContext(ctx)
	feeds := make([]*gofeed.Feed, len(a.Feeds))
	for idx, f := range a.Feeds {
		idx, f := idx, f // copy
		eg.Go(func() error {
			// TODO(later): limit concurrency?
			parsed, err := a.fetchFeed(ctx, &f)
			if err != nil {
				return err
			}
			feeds[idx] = parsed
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	var items []*gofeed.Item
	for _, f := range feeds {
		items = append(items, f.Items...)
	}
	// Sort reverse chronologically by published date
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedParsed.After(*items[j].PublishedParsed)
	})
	return items, nil
}
