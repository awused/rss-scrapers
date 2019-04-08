package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
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

type indexResponse []struct {
	Source       string      `json:"source"`
	Directory    string      `json:"directory"`
	Hash         string      `json:"hash"`
	Height       int         `json:"height"`
	ID           int         `json:"id"`
	Image        string      `json:"image"`
	Change       int         `json:"change"`
	Owner        string      `json:"owner"`
	ParentID     interface{} `json:"parent_id"`
	Rating       string      `json:"rating"`
	Sample       bool        `json:"sample"`
	SampleHeight int         `json:"sample_height"`
	SampleWidth  int         `json:"sample_width"`
	Score        int         `json:"score"`
	Tags         string      `json:"tags"`
	Width        int         `json:"width"`
	FileURL      string      `json:"file_url"`
	CreatedAt    string      `json:"created_at"`
}

type tagListResponse []struct {
	ID        string `json:"id"`
	Tag       string `json:"tag"`
	Count     string `json:"count"`
	Type      string `json:"type"`
	Ambiguous string `json:"ambiguous"`
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
		log.Panic(err)
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title:       strings.Join(os.Args[1:], ", "),
		Link:        &feeds.Link{Href: indexURLPrefix + strings.Join(searchTags, "+")},
		Description: strings.Join(os.Args[1:], ", ") + " - Gelbooru",
	}}

	for _, p := range b {
		createdAt, err := time.Parse(time.RubyDate, p.CreatedAt)
		if err != nil {
			log.Panic(err)
		}
		title := getTitleForImage(db, strings.Split(p.Tags, " "), p.ID, os.Args[1:])
		if title == "" {
			title = strconv.Itoa(p.ID)
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   title,
			Id:      strconv.Itoa(p.ID),
			Link:    &feeds.Link{Href: hrefPrefix + strconv.Itoa(p.ID)},
			Created: createdAt,
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
	db *leveldb.DB, tags []string, id int, searchTags []string) string {
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

func loadMissingTags(db *leveldb.DB, tags []string) {
	resp, err := http.Get(tagsAPIPrefix + strings.Join(tags, "+"))
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
		log.Panic(err)
	}

	for _, tag := range b {
		err = db.Put([]byte(tag.Tag), []byte(tag.Type), nil)
		if err != nil {
			log.Panic(err)
		}
	}
}
