package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gorilla/feeds"
)

const limit = 10

const archiveURL = "https://awkwardzombie.com/comic/archive"
const siteURL = "https://awkwardzombie.com"
const urlPrefix = "https://awkwardzombie.com"

var dateRegex = regexp.MustCompile(`^#(\d+), (\d\d-\d\d-\d\d)$`)

func main() {
	resp, err := http.Get(archiveURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	feed := &feeds.Rss{
		Feed: &feeds.Feed{
			Title: "Awkward Zombie",
			Link:  &feeds.Link{Href: siteURL},
		}}

	divs := htmlquery.Find(doc, "//div[contains(@class, 'archive-line')]")
	if len(divs) > limit {
		divs = divs[:limit]
	}

	for _, d := range divs {
		dateNode := htmlquery.FindOne(d, "//div[contains(@class, 'archive-date')]")
		a := htmlquery.FindOne(d, "//div[contains(@class, 'archive-title')]/a")

		matches := dateRegex.FindStringSubmatch(htmlquery.InnerText(dateNode))
		if matches == nil {
			log.Panicln("Failed to parse date: " + htmlquery.InnerText(dateNode))
		}
		date, err := time.Parse("01-02-06", matches[2])
		if err != nil {
			log.Panicln("Failed to parse date: " + htmlquery.InnerText(dateNode))
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   htmlquery.InnerText(a),
			Id:      matches[1],
			Link:    &feeds.Link{Href: urlPrefix + htmlquery.SelectAttr(a, "href")},
			Created: date,
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
