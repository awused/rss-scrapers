package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

type seriesBody struct {
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

type eventsBody []struct {
	Name         string    `json:"name"`
	Details      string    `json:"details"`
	SeriesType   string    `json:"seriesType"`
	LinkFragment string    `json:"linkFragment"`
	Date         time.Time `json:"date"`
	Attachments  []struct {
		Fullpath   string `json:"fullpath"`
		Size       int    `json:"size"`
		ID         string `json:"id"`
		IsImage    bool   `json:"isImage"`
		Extension  string `json:"extension"`
		ModelType  string `json:"modelType"`
		ForeignKey string `json:"foreignKey"`
		Filename   string `json:"filename"`
	} `json:"attachments"`
	ID string `json:"id"`
}

const urlFormat = `https://api.j-novel.club/api/series/findOne?filter={"where":{"titleslug":"%s"},"include":["volumes","parts"]}`

const eventsUrl = "https://api.j-novel.club/api/events?filter[limit]=100&filter[where][serieId]="
const seriesPage = `https://j-novel.club/s/`
const host = `https://j-novel.club`

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

	var b seriesBody
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Fatalln(err)
	}

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title:       b.Title,
		Link:        &feeds.Link{Href: seriesPage + titleSlug},
		Description: b.DescriptionShort,
	}}

	now := time.Now()

	finalChapters := getFinalChapters(b.ID)

	// TODO -- also include volumes?
	for i := range b.Parts {
		p := b.Parts[len(b.Parts)-i-1]
		if p.Expired {
			continue
		}
		chapterFragment := "/c/" + p.Titleslug
		title := p.Title

		if finalChapters[chapterFragment] {
			title = title + " FINAL"
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   title,
			Id:      p.Titleslug,
			Link:    &feeds.Link{Href: host + chapterFragment},
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

// This only gets recent final chapters but that's good enough
// to tip off a human that the current volume is done.
func getFinalChapters(id string) map[string]bool {
	out := make(map[string]bool)

	resp, err := http.Get(eventsUrl + id)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatalln(err)
	}

	var b eventsBody
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Fatalln(err)
	}

	for _, e := range b {
		if strings.HasSuffix(e.Details, "FINAL") {
			out[e.LinkFragment] = true
		}
	}

	return out
}
