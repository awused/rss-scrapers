package main

// Scrapes trending and popular this week for new stories so you don't have to
// check manually. First run will result in many items but as long as the

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/gorilla/feeds"
)

const popularPages = 5

const trendingUrl = "https://www.royalroad.com/fictions/trending"
const risingUrl = "https://www.royalroad.com/fictions/rising-stars"
const popularUrl = "https://www.royalroad.com/fictions/weekly-popular?page="
const urlPrefix = "https://www.royalroad.com/fiction/"

var idRegex = regexp.MustCompile(`^/fiction/(\d+)(/|$)`)

func getFictions(url string) []*feeds.Item {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// RoyalRoad uses cloudflare but this hasn't been a problem yet
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	anchors := htmlquery.Find(doc, "//h2[contains(@class, 'fiction-title')]/a")
	items := []*feeds.Item{}

	for _, a := range anchors {
		title := htmlquery.InnerText(a)
		href := htmlquery.SelectAttr(a, "href")

		matches := idRegex.FindStringSubmatch(href)
		if matches == nil {
			log.Panicln("Failed to parse Fiction URL: " + href)
		}

		items = append(items, &feeds.Item{
			Title:   title,
			Id:      matches[1],
			Link:    &feeds.Link{Href: urlPrefix + matches[1]},
			Created: time.Now(),
		})
	}

	return items
}

func main() {
	feed := &feeds.Rss{
		Feed: &feeds.Feed{
			Title: "Royal Road - Trending/Popular",
			Link:  &feeds.Link{Href: trendingUrl},
		}}

	feed.Items = append(feed.Items, getFictions(trendingUrl)...)

	feed.Items = append(feed.Items, getFictions(risingUrl)...)

	for i := 0; i < popularPages; i++ {
		pageUrl := popularUrl + strconv.Itoa(i+1)
		// Almost certain to duplicate items, but aw-rss will handle them.
		feed.Items = append(feed.Items, getFictions(pageUrl)...)
	}

	rssFeed := feed.RssFeed()
	// 12 hours, these do not update frequently
	rssFeed.Ttl = 720

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}
