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

const fictionHost = "https://www.fictionpress.com"
const fanfictionHost = "https://www.fanfiction.net"

const storyURLFormat = "%s/s/%s"

// First arg is fiction/fanfiction
// second arg is story ID
func main() {
	if len(os.Args) < 3 {
		log.Panic("Specify fiction/fanfiction and story ID")
	}

	host := ""
	storyID := os.Args[2]

	if os.Args[1] == "fiction" {
		host = fictionHost
	} else if os.Args[1] == "fanfiction" {
		host = fanfictionHost
	} else {
		log.Panic("Specify fiction/fanfiction")
	}

	storyURL := fmt.Sprintf(storyURLFormat, host, storyID)

	resp, err := http.Get(storyURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	title := htmlquery.InnerText(
		htmlquery.FindOne(doc, "//div[@id='profile_top']/b"))

	feed := &feeds.Rss{&feeds.Feed{
		Title: title,
		Link:  &feeds.Link{Href: storyURL},
	}}

	now := time.Now()

	// htmlquery doesn't handle this properly if it's just one expression
	selectEl := htmlquery.FindOne(doc, "//select[@id='chap_select'][1]")
	chapters := htmlquery.Find(selectEl, "option")

	for i := range chapters {
		chap := chapters[len(chapters)-i-1]

		text := htmlquery.InnerText(chap)
		value := htmlquery.SelectAttr(chap, "value")

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   text,
			Id:      value,
			Link:    &feeds.Link{Href: storyURL + "/" + value},
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
