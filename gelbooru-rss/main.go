package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

const indexURLPrefix = "https://gelbooru.com/index.php?page=post&s=list&tags="
const indexAPIPrefix = "https://gelbooru.com/index.php?page=dapi&s=post&q=index&json=1&tags="
const tagsAPIPrefix = "https://gelbooru.com/index.php?page=dapi&s=tag&q=index&json=1&names="
const hrefPrefix = "https://gelbooru.com/index.php?page=post&s=view&id="
const gelbooruRoot = "https://gelbooru.com/"

type indexResponse struct {
	Post []struct {
		ID          int    `json:"id"`
		CreatedAt   string `json:"created_at"`
		Md5         string `json:"md5"`
		Image       string `json:"image"`
		Tags        string `json:"tags"`
		Status      string `json:"status"`
		HasChildren string `json:"has_children"`
	} `json:"post"`
}

type tagListResponse struct {
	Tag []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Count     int    `json:"count"`
		Type      int    `json:"type"`
		Ambiguous int    `json:"ambiguous"`
	} `json:"tag"`
}

func main() {
	if len(os.Args) < 2 {
		log.Panic("Specify at least one tag")
	}

	db := openTagsDB()
	defer db.Close()

	searchTags := []string{}
	for _, v := range os.Args[1:] {
		searchTags = append(searchTags, url.QueryEscape(v))
	}

	indexURL := indexAPIPrefix + strings.Join(searchTags, "+")

	resp, err := http.Get(indexURL)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Panic(err)
	}

	var b indexResponse
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Panic(indexURL, "\n", err)
	}

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title:       strings.Join(os.Args[1:], ", "),
		Link:        &feeds.Link{Href: indexURLPrefix + strings.Join(searchTags, "+")},
		Description: strings.Join(os.Args[1:], ", ") + " - Gelbooru",
	}}

	for _, p := range b.Post {
		id := strconv.Itoa(p.ID)
		createdAt, err := time.Parse(time.RubyDate, p.CreatedAt)
		if err != nil {
			log.Panic(err)
		}
		title := getTitleForImage(db, strings.Split(p.Tags, " "), id, os.Args[1:])
		if title == "" {
			title = id
		}
		title = title + " - " + p.Md5

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   title,
			Id:      id,
			Link:    &feeds.Link{Href: hrefPrefix + id},
			Created: createdAt,
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

func openTagsDB() *leveldb.DB {
	home := os.Getenv("HOME")
	if home == "" {
		log.Panic("Empty home")
	}

	dir := path.Join(home, ".rss", "geltagdb")

	db, err := leveldb.OpenFile(dir, nil)
	if errors.IsCorrupted(err) {
		db, err = leveldb.RecoverFile(dir, nil)
	}
	if err != nil {
		log.Panic(err)
	}
	return db
}

// May return an empty string if there's no real important tags
func getTitleForImage(
	db *leveldb.DB, tags []string, id string, searchTags []string) string {
	unsatisfiedTags := []string{}
	relevantTags := []string{}

	for _, t := range tags {
		kind, err := db.Get([]byte(t), nil)
		if err == leveldb.ErrNotFound {
			unsatisfiedTags = append(unsatisfiedTags, t)
		} else if err != nil {
			log.Panic(err)
		} else {
			if includeTag(t, string(kind), searchTags) {
				relevantTags = append(relevantTags, t)
			}
		}
	}

	if len(unsatisfiedTags) > 0 {
		for i := 0; i < len(unsatisfiedTags); i += 50 {
			j := i + 50
			if j > len(unsatisfiedTags) {
				j = len(unsatisfiedTags)
			}
			loadMissingTags(db, unsatisfiedTags[i:j])
		}

		for _, t := range unsatisfiedTags {
			kind, err := db.Get([]byte(t), nil)
			if err == leveldb.ErrNotFound {
			} else if err != nil {
				log.Panic(err)
			} else {
				if includeTag(t, string(kind), searchTags) {
					relevantTags = append(relevantTags, t)
				}
			}
		}
	}

	return strings.Join(relevantTags, ", ")
}

func includeTag(tag string, kind string, searchTags []string) bool {
	if kind != "character" && kind != "copyright" && kind != "artist" {
		return false
	}

	for _, v := range searchTags {
		if tag == v {
			return false
		}
	}

	return true
}

var tagTypes = map[int]string{
	0: "tag",
	1: "artist",
	3: "copyright",
	4: "character",
	5: "metadata",
	6: "deprecated",
}

func loadMissingTags(db *leveldb.DB, tags []string) {
	escaped := []string{}

	for _, t := range tags {
		// Yes this is as dumb as it looks
		escaped = append(escaped, url.QueryEscape(html.UnescapeString(html.UnescapeString(t))))
	}

	url := tagsAPIPrefix + strings.Join(escaped, "+")

	resp, err := http.Get(url)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Panic(err)
	}

	var b tagListResponse
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Panic(url, "\n", err)
	}

	for _, tag := range b.Tag {
		// Still as dumb as it looks
		err = db.Put([]byte(html.EscapeString(tag.Name)), []byte(tagTypes[tag.Type]), nil)
		if err != nil {
			log.Panic(err)
		}
	}
}
