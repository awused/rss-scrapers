package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/awused/awconf"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

// TODO -- maximum complexity of a search
// TODO -- time limit
// TODO -- Rename as a more general Nyaa rss tool

// As far as I can tell two structures is the only type-safe way to do this.
type config map[string][]string

type titleConfig struct {
	Title string
}

var quarterRe = regexp.MustCompile(`^(\d{4})[Qq]([1-4])$`)

func main() {
	flag.Parse()

	var conf config
	var titleConf titleConfig
	awconf.LoadConfig("seasonal-anime-rss", &conf)
	awconf.LoadConfig("seasonal-anime-rss", &titleConf)

	searches := []string{}

	for _, v := range conf["ongoing"] {
		searches = append(searches, v)
	}

	now := time.Now()

	for quarter, quarterSearches := range conf {
		matches := quarterRe.FindStringSubmatch(quarter)

		if matches == nil {
			continue
		}

		y, _ := strconv.Atoi(matches[1]) // Cannot fail
		q, _ := strconv.Atoi(matches[2]) // Cannot fail

		eoq := time.Date(y, time.Month(q*3+1), 0, 0, 0, 0, 0, time.UTC)
		eoq = eoq.Add(14 * 24 * time.Hour)
		if eoq.After(now) {
			for _, s := range quarterSearches {
				searches = append(searches, s)
			}
		}
	}

	query := ""
	if len(searches) != 0 {
		query = url.QueryEscape("(" + strings.Join(searches, ")|(") + ")")
	}

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title: titleConf.Title,
		Link:  &feeds.Link{Href: "https://nyaa.si/?q=" + query},
	}}

	if len(searches) != 0 {
		parser := gofeed.NewParser()
		parsed, err := parser.ParseURL("https://nyaa.si/?page=rss&q=" + query)
		if err != nil {
			log.Panic(err)
		}

		for _, gfi := range parsed.Items {
			feed.Items = append(feed.Items, &feeds.Item{
				Title:       gfi.Title,
				Link:        &feeds.Link{Href: gfi.Link},
				Created:     *gfi.PublishedParsed,
				Description: gfi.Description,
				Id:          gfi.GUID,
			})
		}
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 60

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}
