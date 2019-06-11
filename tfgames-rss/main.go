package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gorilla/feeds"
)

const urlPrefix = "https://tfgames.site/?module=viewgame&id="
const fanfictionHost = "https://www.fanfiction.net"

const storyURLFormat = "%s/s/%s"

// First arg is fiction/fanfiction
// second arg is story ID
func main() {
	if len(os.Args) < 2 {
		log.Panic("Specify game ID")
	}

	id := os.Args[1]

	pageURL := urlPrefix + id

	resp, err := http.Get(pageURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	title := htmlquery.InnerText(
		htmlquery.FindOne(doc, "//title"))

	feed := &feeds.Rss{&feeds.Feed{
		Title: title,
		Link:  &feeds.Link{Href: pageURL},
	}}

	now := time.Now()

	// htmlquery doesn't handle this properly if it's just one expression
	downloadEls := htmlquery.Find(doc, "//div[@id='downloads']/*")

	version := ""

	for _, e := range downloadEls {
		if e.Data == "center" {
			version = htmlquery.InnerText(e)
			continue
		}
		if htmlquery.SelectAttr(e, "class") != "dlcontainer" {
			continue
		}

		a := htmlquery.FindOne(e, "//div[@class='dltext']/a")
		aText := htmlquery.InnerText(a)
		href := htmlquery.SelectAttr(a, "href")

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   version + " " + aText,
			Id:      href,
			Link:    &feeds.Link{Href: href},
			Created: now,
		})
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 360

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}
