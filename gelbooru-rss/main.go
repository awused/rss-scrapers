package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/syndtr/goleveldb/leveldb"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
)

const indexURLPrefix = "https://gelbooru.com/index.php?page=post&s=list&tags="
const indexAPIFormat = "https://gelbooru.com/index.php?page=dapi%s&s=post&q=index&json=1&tags=%s"
const tagsAPIFormat = "https://gelbooru.com/index.php?page=dapi%s&s=tag&q=index&json=1&names=%s"
const hrefPrefix = "https://gelbooru.com/index.php?page=post&s=view&id="
const gelbooruRoot = "https://gelbooru.com/"

var apiKey = ""

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

	apiKey = loadApiKey()

	db := openTagsDB()
	defer db.Close()

	blacklist := loadBlacklist()

	searchTags := []string{}
	for _, v := range os.Args[1:] {
		searchTags = append(searchTags, url.QueryEscape(v))
	}

	indexURL := fmt.Sprintf(indexAPIFormat, apiKey, strings.Join(searchTags, "+"))

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
		log.Panic(indexURL, "\n", err, "\n", string(body))
	}

	matchedBlacklisted := make(map[string]bool)

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title:       strings.Join(os.Args[1:], ", "),
		Link:        &feeds.Link{Href: indexURLPrefix + strings.Join(searchTags, "+")},
		Description: strings.Join(os.Args[1:], ", ") + " - Gelbooru",
	}}

outer:
	for _, p := range b.Post {
		id := strconv.Itoa(p.ID)
		createdAt, err := time.Parse(time.RubyDate, p.CreatedAt)
		if err != nil {
			log.Panic(err)
		}

		tags := strings.Split(p.Tags, " ")
		if len(blacklist) > 0 {
			for _, t := range tags {
				if blacklist[t] {
					matchedBlacklisted[t] = true
					continue outer
				}
			}
		}

		title := getTitleForImage(db, tags, id, os.Args[1:])
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

	blacklistSorted := []string{}
	for t := range matchedBlacklisted {
		blacklistSorted = append(blacklistSorted, t)
	}
	sort.Slice(blacklistSorted, func(i, j int) bool {
		return blacklistSorted[i] < blacklistSorted[j]
	})

	blacklistString := strings.Join(blacklistSorted, "+-")

	if len(blacklistString) != 0 {
		feed.Link.Href += "+-" + blacklistString
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 120

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
	if ldberrors.IsCorrupted(err) {
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

	url := fmt.Sprintf(tagsAPIFormat, apiKey, strings.Join(escaped, "+"))

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

func loadApiKey() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	fpath := path.Join(home, ".rss", "gelapi")

	file, err := os.Open(fpath)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		log.Panic("API key file exists but cannot be read", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	return scanner.Text()
}

func loadBlacklist() map[string]bool {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}

	fpath := path.Join(home, ".rss", "gelblacklist")

	file, err := os.Open(fpath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		log.Panic("Tag blacklist exists but cannot be read", err)
	}
	defer file.Close()

	out := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			out[line] = true
		}
	}

	return out
}
