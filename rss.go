package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/feeds"
)

type RSSProvider struct {
}

func NewRSSProvider() *RSSProvider {
	return &RSSProvider{}
}

func (p *RSSProvider) Feed(topicID int) (*feeds.Feed, error) {
	downloader := NewPostDownloader()
	title, posts, err := downloader.Download(topicID)
	if err != nil {
		return nil, err
	}

	feed := &feeds.Feed{
		Title: title,
		Link:  &feeds.Link{},
		Id:    fmt.Sprintf("4pda-%d", topicID),
	}

	for _, post := range posts {
		mainText := post.MainText
		if utf8.RuneCountInString(mainText) > 50 {
			words := strings.Fields(mainText)
			if len(words) > 15 {
				words = words[:15]
			}
			mainText = strings.Join(words, " ")
		}

		postID := post.Link
		if postID == "" {
			postID = strconv.FormatInt(post.ID, 10)
		}
		if !post.Updated.IsZero() {
			postID += "#" + strconv.FormatInt(post.Updated.Unix(), 10)
		}
		item := &feeds.Item{
			Title: mainText,
			Link: &feeds.Link{
				Href: post.Link,
			},
			Id:          postID,
			Created:     post.Created,
			Updated:     post.Updated,
			Description: post.HTML,
		}
		feed.Items = append(feed.Items, item)
	}

	return feed, nil
}
