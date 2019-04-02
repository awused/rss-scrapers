package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/awused/awconf"
	"github.com/gorilla/feeds"
)

type config struct {
	MemberID string
	PassHash string
}

const urlPrefix = "https://exhentai.org/?"
const cookieFormat = "ipb_pass_hash=%s; ipb_member_id=%s;"

func main() {
	var conf config
	awconf.LoadConfig("panda-rss", &conf)

	searchURL := urlPrefix
	title := "Exhentai"
	if len(os.Args) > 1 {
		searchURL += os.Args[1]
		vals, err := url.ParseQuery(searchURL)
		if err != nil {
			log.Panic(err)
		}

		if q := vals.Get("f_search"); q != "" {
			title += " " + strings.Join(strings.Split(q, "+"), ", ")
		}
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title: title,
		Link:  &feeds.Link{Href: searchURL},
	}}

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Panic(err)
	}

	//req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:66.0) Gecko/20100101 Firefox/66.0")
	req.Header.Add("Cookie", fmt.Sprintf(cookieFormat, conf.PassHash, conf.MemberID))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	for i, a := range htmlquery.Find(doc, "//td[contains(@class, 'glname')]//a") {
		fmt.Println(i)
		fmt.Println(htmlquery.InnerText(a))
		fmt.Println(htmlquery.SelectAttr(a, "href"))

		div := htmlquery.FindOne(a, "ancestor::td[contains(@class, 'glname')]/..//div[contains(@id, 'posted_')]")
		posted, err := time.Parse("2006-01-02 15:04", htmlquery.InnerText(div))
		if err != nil {
			log.Panic(err)
		}

		id := strings.TrimPrefix(htmlquery.SelectAttr(div, "id"), "posted_")

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   htmlquery.InnerText(a),
			Id:      id,
			Link:    &feeds.Link{Href: htmlquery.SelectAttr(a, "href")},
			Created: posted,
		})
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 60

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}
