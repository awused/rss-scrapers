package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

const urlPrefix = "https://jumpg-webapi.tokyo-cdn.com/api/title_detail?title_id="

const seriesPrefix = "https://mangaplus.shueisha.co.jp/titles/"
const chapterPrefix = "https://mangaplus.shueisha.co.jp/viewer/"

const chapterRe = `/chapter/([0-9]+)/`
const chapterTitleRe = `#[0-9]+"([^*]+)\*`
const titleRe = `#MANGA_Plus ([^@]+)@`

var chapterRegex = regexp.MustCompile(chapterRe)
var chapterTitleRegex = regexp.MustCompile(chapterTitleRe)
var titleRegex = regexp.MustCompile(titleRe)

func main() {
	if len(os.Args) < 2 {
		log.Panic("Specify a series")
	}

	series := os.Args[1]

	detailsURL := urlPrefix + series

	resp, err := http.Get(detailsURL)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	byteBody, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Panic(err)
	}
	body := string(byteBody)

	matches := titleRegex.FindStringSubmatch(body)
	if matches == nil {
		log.Panic("Could not find manga title")
	}
	mangaTitle := strip(matches[1])

	chapterTitleMatches := chapterTitleRegex.FindAllStringSubmatch(body, -1)
	chapterMatches := chapterRegex.FindAllStringSubmatch(body, -1)

	if len(chapterMatches) == 0 {
		return
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title: mangaTitle,
		Link:  &feeds.Link{Href: seriesPrefix + series},
		// The description exists in the response but ehhhhh
		Description: mangaTitle,
	}}

	now := time.Now()

	for ind := range chapterMatches {
		i := len(chapterMatches) - ind - 1
		chapterNumber := strip(chapterMatches[i][1])
		chapterTitle := chapterNumber
		if len(chapterTitleMatches) > i {
			chapterTitle = strip(chapterTitleMatches[i][1])
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   chapterTitle,
			Id:      chapterNumber,
			Link:    &feeds.Link{Href: chapterPrefix + chapterNumber},
			Created: now,
		})
	}

	rssFeed := feed.RssFeed()
	// 3 hour TTL, these don't update a lot and it's a waste of bandwidth since
	// they don't use any compression
	rssFeed.Ttl = 180

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}

func strip(str string) string {
	return strings.TrimSpace(
		strings.Map(func(r rune) rune {
			if r >= 32 && r != 127 {
				return r
			}
			return -1
		}, str))
}
