package plnt

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

type Feed struct {
	ShortName string // used in the state directory name, e.g. “sur5r-blog”
	Title     string // human-readable, e.g. “sur5r’s Hardware Blog”
	URL       string
}

type Aggregator struct {
	Feeds []Feed
	// TODO: option to force loading everything from cache for development
}

func (a *Aggregator) fromCache(feed *Feed) error {
	return nil
}

func (a *Aggregator) fetchFeed(ctx context.Context, feed *Feed) (*gofeed.Feed, error) {
	// TODO: use cache modtime and set if-modified-since to skip transfer/parse/normalize
	log.Printf("[%s] fetching %s", feed.ShortName, feed.URL)
	req, err := http.NewRequest("GET", feed.URL, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err // TODO: fallback
	}
	defer resp.Body.Close()
	parser := gofeed.NewParser()
	f, err := parser.Parse(resp.Body)
	if err != nil {
		return nil, err // TODO: fallback
	}
	// Exhaust the reader to make HTTP connection pooling/keepalive work:
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	// TODO: parse feed and spit it out into a bytes buffer, persist that
	// normalized version (ensures we can parse the cached version in later
	// invocations)

	items := make([]*gofeed.Item, 0, len(f.Items))
	for _, i := range f.Items {
		if i.PublishedParsed == nil {
			log.Printf("[%s] dropping post %v: no published date", feed.ShortName, i) // TODO: post title
			continue
		}
		items = append(items, i)
	}
	f.Items = items

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
