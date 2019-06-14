// Package f2f converts from the gofeed parser types to the gorilla/feeds types.
package f2f

import (
	"time"

	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

func GofeedToGorillaFeed(f *gofeed.Item) *feeds.Item {
	var author *feeds.Author
	if f.Author != nil {
		author = &feeds.Author{
			Name:  f.Author.Name,
			Email: f.Author.Email,
		}
	}
	var updated, created time.Time
	if f.UpdatedParsed != nil {
		updated = *f.UpdatedParsed
	}
	if f.PublishedParsed != nil {
		created = *f.PublishedParsed
	}
	return &feeds.Item{
		Title: f.Title,
		Link:  &feeds.Link{Href: f.Link},
		// TODO: source field?
		Author:      author,
		Description: f.Description,
		Id:          f.GUID,
		Updated:     updated,
		Created:     created,
		// TODO: convert enclosures
		Content: f.Content,
	}
}
