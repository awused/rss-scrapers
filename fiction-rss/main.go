package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gorilla/feeds"
)

const host = "https://archiveofourown.org"
const indexURLFormat = host + "/works/%s/navigate"

// Arg is work ID
func main() {
	if len(os.Args) < 2 {
		log.Panic("Specify story ID")
	}

	id := os.Args[1]
	indexURL := fmt.Sprintf(indexURLFormat, id)

	resp, err := http.Get(indexURL)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	byteBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Panic(err)
	}

	doc, err := htmlquery.Parse(bytes.NewReader(byteBody))
	if err != nil {
		log.Panic(err)
	}

	title := htmlquery.InnerText(
		htmlquery.FindOne(doc, "//h2[@class='heading']/a[1]"))

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title: title,
		Link:  &feeds.Link{Href: indexURL},
	}}

	// Page does contain dates but not times, using "now" blindly is easier and ensures correct sorting of new items.
	now := time.Now()

	chapters := htmlquery.Find(doc, "//ol[contains(@class, 'chapter')]/li/a")

	for i := range chapters {
		chap := chapters[len(chapters)-i-1]

		text := htmlquery.InnerText(chap)
		href := htmlquery.SelectAttr(chap, "href")

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   text,
			Id:      href,
			Link:    &feeds.Link{Href: host + href},
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
