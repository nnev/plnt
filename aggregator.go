package plnt

import (
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
	"time"

	"github.com/google/renameio"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

type Feed struct {
	ShortName string // used in the state directory name, e.g. “sur5r-blog”
	Title     string // human-readable, e.g. “sur5r’s Hardware Blog”
	URL       string
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

func (a *Aggregator) from(r io.Reader, feed *Feed) (*gofeed.Feed, error) {
	parser := gofeed.NewParser()
	f, err := parser.Parse(r)
	if err != nil {
		return nil, err
	}

	items := make([]*gofeed.Item, 0, len(f.Items))
	for _, i := range f.Items {
		if i.PublishedParsed == nil {
			log.Printf("[%s] dropping post %v: no published date", feed.ShortName, i) // TODO: post title
			continue
		}
		i.Title = fmt.Sprintf("[%s] %s", feed.Title, i.Title)
		items = append(items, i)
	}
	f.Items = items

	return f, nil
}

func (a *Aggregator) fromCache(feed *Feed) (*gofeed.Feed, error) {
	b, err := ioutil.ReadFile(feed.cachePath())
	if err != nil {
		return nil, err
	}
	var f gofeed.Feed
	return &f, json.Unmarshal(b, &f)
}

func (a *Aggregator) fetchFeed(ctx context.Context, feed *Feed) (*gofeed.Feed, error) {
	if a.ForceFromCache {
		return a.fromCache(feed)
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
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[%s] falling back to cache due to fetch error: %v", feed.ShortName, err)
		return a.fromCache(feed)
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		if resp.StatusCode == http.StatusNotModified {
			return a.fromCache(feed)
		}
		log.Printf("[%s] falling back to cache due to fetch error: unexpected status code: got %v, want %v", feed.ShortName, got, want)
		return a.fromCache(feed)
	}
	f, err := a.from(resp.Body, feed)
	if err != nil {
		log.Printf("[%s] falling back to cache due to fetch error: %v", feed.ShortName, err)
		return a.fromCache(feed)
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
