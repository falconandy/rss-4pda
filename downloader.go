package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	topicURLPattern = "http://4pda.ru/forum/index.php?showtopic=%d"
	pageSize        = 20
	maxPostCount    = 30
)

type PostDownloader struct {
	client *http.Client
}

func NewPostDownloader() *PostDownloader {
	return &PostDownloader{
		client: &http.Client{},
	}
}

func (d *PostDownloader) Download(topicID int) (title string, posts []Post, err error) {
	firstPosts, lastFrom, title, err := d.downloadPosts(topicID, 0)
	if err != nil {
		return "", nil, err
	}
	if lastFrom == -1 {
		return title, firstPosts, nil
	}

	from := lastFrom
	for from >= 0 && len(posts) < maxPostCount {
		var pagePosts []Post
		if from <= 0 {
			pagePosts = firstPosts
		} else {
			pagePosts, _, _, err = d.downloadPosts(topicID, from)
			if err != nil {
				return "", nil, err
			}
		}
		posts = append(pagePosts, posts...)
		from -= pageSize
	}

	if len(posts) > maxPostCount {
		posts = posts[len(posts)-maxPostCount:]
	}

	return title, posts, nil
}

func (d *PostDownloader) downloadPosts(topicID, from int) (posts []Post, lastFrom int, title string, err error) {
	pageFrom := from
	for {
		var pagePosts []Post
		pagePosts, lastFrom, title, err = d.downloadPagePosts(topicID, pageFrom)
		if err != nil {
			return nil, -1, "", err
		}
		if len(pagePosts) == 0 {
			break
		}
		posts = append(posts, pagePosts...)
		if len(posts) > pageSize {
			posts = posts[:pageSize]
			break
		}
		pageFrom += len(pagePosts)
	}
	return posts, lastFrom, title, nil
}

func (d *PostDownloader) downloadPagePosts(topicID, from int) (posts []Post, lastFrom int, title string, err error) {
	content, contentType, err := d.downloadTopicPage(topicID, from)
	if err != nil {
		return nil, -1, "", err
	}
	if contentType == "" {
		contentType = "text/html"
	}

	contentEncoding, _, _ := charset.DetermineEncoding(content, contentType)
	decoder := contentEncoding.NewDecoder()

	doc, err := goquery.NewDocumentFromReader(decoder.Reader(bytes.NewReader(content)))
	if err != nil {
		return nil, -1, "", errors.Wrapf(err, "can't parse content for topic %d", topicID)
	}

	lastFrom = d.findLastPageFrom(doc)
	title, posts = d.findPosts(doc, from == 0)

	return posts, lastFrom, title, nil
}

func (d *PostDownloader) downloadTopicPage(topicID, from int) (content []byte, contentType string, err error) {
	pageURL := d.pageURL(topicID, from)
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", errors.Wrapf(err, "can't create an http request for %s", pageURL)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	r, err := d.client.Do(req)
	if err != nil {
		return nil, "", errors.Wrapf(err, "can't download from %s", pageURL)
	}
	defer func() { _ = r.Body.Close() }()

	if r.StatusCode != http.StatusOK {
		return nil, "", errors.Errorf("can't download %s, unexpected status code %s", pageURL, r.Status)
	}

	var reader io.ReadCloser
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(r.Body)
		if err != nil {
			return nil, "", errors.Wrapf(err, "can't create a gzip reader for %s response", pageURL)
		}
		defer func() { _ = reader.Close() }()
	default:
		reader = r.Body
	}

	content, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, "", errors.Wrapf(err, "can't read the body of %s response", pageURL)
	}
	return content, r.Header.Get("Content-Type"), nil
}

func (d *PostDownloader) findLastPageFrom(doc *goquery.Document) int {
	from := -1
	doc.Find("div.pagination a").Each(func(i int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		if href != "" {
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			if u.Query().Get("st") != "" {
				stValue, err := strconv.Atoi(u.Query().Get("st"))
				if err == nil && from < stValue {
					from = stValue
				}
			}
		}
	})
	return from
}

func (d *PostDownloader) findPosts(doc *goquery.Document, includePinned bool) (title string, posts []Post) {
	doc.Find("head").Each(func(i int, s *goquery.Selection) {
		title = s.Find("title").Text()
	})

	doc.Find("div[data-post]").Each(func(i int, s *goquery.Selection) {
		_, isPinned := s.Parent().Attr("data-spoil-poll-pinned-content")
		if !includePinned && isPinned {
			return
		}

		dataPost := s.AttrOr("data-post", "")
		id, err := strconv.ParseInt(dataPost, 10, 64)
		if err != nil {
			log.Printf("can't parse post id: %s", dataPost)
		}
		post := Post{
			ID: id,
		}
		s.Find("div.post_header_container div.post_header span.post_date").Each(func(i int, s *goquery.Selection) {
			parts := strings.Split(s.Text(), "|")
			if len(parts) == 2 {
				if posted, ok := d.parseDate(parts[0]); ok {
					post.Created = posted
				}
			}

			s.Find("a").Each(func(i int, s *goquery.Selection) {
				if post.Link == "" {
					postLink := s.AttrOr("href", "")
					if strings.HasPrefix(postLink, "//") {
						postLink = "http:" + postLink
					}
					post.Link = postLink
				}
			})
		})
		s.Find("div.post_body").Each(func(i int, s *goquery.Selection) {
			s.Find("div.quote").Each(func(i int, s *goquery.Selection) {
				s.SetAttr("style", "border-left: 2px solid lightgrey; padding-left: 5px; margin-bottom: 5px; color: grey;")
			})

			htmlContent, err := s.Html()
			if err == nil {
				post.HTML = emoji.Sprint(htmlContent)
			}

			var mainText bytes.Buffer
			for _, n := range s.Nodes {
				child := n.FirstChild
				for child != nil {
					if child.Type == html.TextNode {
						mainText.WriteString(child.Data)
					}
					child = child.NextSibling
				}
			}
			post.MainText = strings.TrimSpace(strings.TrimLeftFunc(emoji.Sprint(mainText.String()), func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsDigit(r)
			}))

			s.Find("span.edit").Each(func(i int, s *goquery.Selection) {
				parts := strings.Split(s.Text(), " - ")
				if len(parts) > 0 {
					if updated, ok := d.parseDate(parts[len(parts)-1]); ok {
						post.Updated = updated
					}
				}
			})
		})
		posts = append(posts, post)
	})
	return title, posts
}

func (d *PostDownloader) pageURL(topicID, from int) string {
	result := fmt.Sprintf(topicURLPattern, topicID)
	if from > 0 {
		result += fmt.Sprintf("&st=%d", from)
	}
	return result
}

func (d *PostDownloader) parseDate(dateStr string) (time.Time, bool) {
	dateStr = strings.ToLower(strings.TrimSpace(dateStr))
	switch {
	case strings.HasPrefix(dateStr, "сегодня, "):
		dateStr = time.Now().Format("02.01.06") + strings.TrimPrefix(dateStr, "сегодня")
	case strings.HasPrefix(dateStr, "вчера, "):
		dateStr = time.Now().Add(-24*time.Hour).Format("02.01.06") + strings.TrimPrefix(dateStr, "вчера")
	}

	if date, err := time.ParseInLocation("02.01.06, 15:04", dateStr, time.Local); err == nil {
		return date, true
	}
	return time.Time{}, false
}
