package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/feeds"
)

type responseBody struct {
	Title               string    `json:"title"`
	Titleslug           string    `json:"titleslug"`
	TitleShort          string    `json:"titleShort"`
	TitleOriginal       string    `json:"titleOriginal"`
	Author              string    `json:"author"`
	Illustrator         string    `json:"illustrator"`
	AuthorOriginal      string    `json:"authorOriginal"`
	IllustratorOriginal string    `json:"illustratorOriginal"`
	Translator          string    `json:"translator"`
	Editor              string    `json:"editor"`
	Description         string    `json:"description"`
	DescriptionShort    string    `json:"descriptionShort"`
	Tags                string    `json:"tags"`
	ForumLink           string    `json:"forumLink"`
	Created             time.Time `json:"created"`
	OverrideExpiration  bool      `json:"override_expiration"`
	Attachments         []struct {
		Fullpath   string `json:"fullpath"`
		Size       int    `json:"size"`
		ID         string `json:"id"`
		IsImage    bool   `json:"isImage"`
		Extension  string `json:"extension"`
		ModelType  string `json:"modelType"`
		ForeignKey string `json:"foreignKey"`
		Filename   string `json:"filename"`
	} `json:"attachments"`
	ID        string `json:"id"`
	Postcount int    `json:"postcount"`
	Volumes   []struct {
		Title               string    `json:"title"`
		Titleslug           string    `json:"titleslug"`
		TitleShort          string    `json:"titleShort"`
		TitleOriginal       string    `json:"titleOriginal"`
		VolumeNumber        int       `json:"volumeNumber"`
		Author              string    `json:"author"`
		Illustrator         string    `json:"illustrator"`
		AuthorOriginal      string    `json:"authorOriginal"`
		IllustratorOriginal string    `json:"illustratorOriginal"`
		Translator          string    `json:"translator"`
		Editor              string    `json:"editor"`
		Description         string    `json:"description"`
		DescriptionShort    string    `json:"descriptionShort"`
		PublisherOriginal   string    `json:"publisherOriginal"`
		Label               string    `json:"label"`
		PublishingDate      time.Time `json:"publishingDate"`
		Tags                string    `json:"tags"`
		ForumLink           string    `json:"forumLink"`
		Created             time.Time `json:"created"`
		Attachments         []struct {
			Fullpath   string `json:"fullpath"`
			Size       int    `json:"size"`
			ID         string `json:"id"`
			IsImage    bool   `json:"isImage"`
			Extension  string `json:"extension"`
			ModelType  string `json:"modelType"`
			ForeignKey string `json:"foreignKey"`
			Filename   string `json:"filename"`
		} `json:"attachments"`
		ID        string `json:"id"`
		SerieID   string `json:"serieId"`
		Postcount int    `json:"postcount"`
	} `json:"volumes"`
	Parts []struct {
		Title               string    `json:"title"`
		Titleslug           string    `json:"titleslug"`
		TitleShort          string    `json:"titleShort"`
		TitleOriginal       string    `json:"titleOriginal"`
		Author              string    `json:"author"`
		Illustrator         string    `json:"illustrator"`
		AuthorOriginal      string    `json:"authorOriginal"`
		IllustratorOriginal string    `json:"illustratorOriginal"`
		Translator          string    `json:"translator"`
		Editor              string    `json:"editor"`
		PartNumber          int       `json:"partNumber"`
		Description         string    `json:"description"`
		DescriptionShort    string    `json:"descriptionShort"`
		Tags                string    `json:"tags"`
		LaunchDate          time.Time `json:"launchDate"`
		ExpirationDate      time.Time `json:"expirationDate"`
		Expired             bool      `json:"expired"`
		Preview             bool      `json:"preview"`
		ForumLink           string    `json:"forumLink"`
		BannedCountries     string    `json:"bannedCountries"`
		Created             time.Time `json:"created"`
		Attachments         []struct {
			Fullpath   string `json:"fullpath"`
			Size       int    `json:"size"`
			ID         string `json:"id"`
			IsImage    bool   `json:"isImage"`
			Extension  string `json:"extension"`
			ModelType  string `json:"modelType"`
			ForeignKey string `json:"foreignKey"`
			Filename   string `json:"filename"`
		} `json:"attachments"`
		ID        string `json:"id"`
		VolumeID  string `json:"volumeId"`
		SerieID   string `json:"serieId"`
		Postcount int    `json:"postcount"`
	} `json:"parts"`
}

const urlFormat = `https://api.j-novel.club/api/series/findOne?filter={"where":{"titleslug":"%s"},"include":["volumes","parts"]}`

const seriesPage = `https://j-novel.club/s/`
const chapterPage = `https://j-novel.club/c/`

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Specify titleslug")
	}

	titleSlug := os.Args[1]

	url := fmt.Sprintf(urlFormat, titleSlug)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatalln(err)
	}

	var b responseBody
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Fatalln(err)
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title:       b.Title,
		Link:        &feeds.Link{Href: seriesPage + titleSlug},
		Description: b.DescriptionShort,
	}}

	now := time.Now()

	// TODO -- also include volumes?
	for i := range b.Parts {
		p := b.Parts[len(b.Parts)-i-1]
		if p.Expired {
			continue
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   p.Title,
			Id:      p.Titleslug,
			Link:    &feeds.Link{Href: chapterPage + p.Titleslug},
			Created: now,
		})
	}

	rssFeed := feed.RssFeed()
	rssFeed.Ttl = 60

	feedXml, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Print(string(feedXml))
}
