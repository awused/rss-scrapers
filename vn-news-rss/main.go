package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gorilla/feeds"
)

const url = "https://erogegames.com/eroge-visual-novels/eroge-news/"

func main() {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title: "Visual Novel Translation Status",
		Link:  &feeds.Link{Href: url},
	}}

	now := time.Now()

	for _, a := range htmlquery.Find(doc, "//a[contains(@id, 'thread_title_')]") {

		text := htmlquery.InnerText(a)
		if strings.Contains(text, "H-RPG") {
			continue
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   text,
			Id:      htmlquery.SelectAttr(a, "id"),
			Link:    &feeds.Link{Href: htmlquery.SelectAttr(a, "href")},
			Created: now,
		})
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 300

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}
