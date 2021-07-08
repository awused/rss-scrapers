package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/feeds"
	log "github.com/sirupsen/logrus"
)

// Hackily slapped together from code from manga-syncer
// Hopefully this doesn't need to function forever.

const mangaURL = "https://api.mangadex.org/manga/%s"
const delay = 2 * time.Second
const pageSize = 100

var client *http.Client = &http.Client{
	Transport: &http.Transport{
		IdleConnTimeout: 30 * time.Second,
	},
}

type stringable string

func (st *stringable) UnmarshalJSON(b []byte) error {
	if b[0] != '"' {
		var i int
		err := json.Unmarshal(b, &i)
		*st = (stringable)(strconv.Itoa(i))
		return err
	}
	return json.Unmarshal(b, (*string)(st))
}

type mangaChapter struct {
	Result string `json:"result"`
	Data   struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Volume             *stringable `json:"volume"`
			Chapter            stringable  `json:"chapter"`
			Title              *string     `json:"title"`
			TranslatedLanguage string      `json:"translatedLanguage"`
			Hash               string      `json:"hash"`
			Data               []string    `json:"data"`
			DataSaver          []string    `json:"dataSaver"`
			PublishAt          time.Time   `json:"publishAt"`
			CreatedAt          time.Time   `json:"createdAt"`
			UpdatedAt          interface{} `json:"updatedAt"`
			Version            int         `json:"version"`
		} `json:"attributes"`
	} `json:"data"`
	Relationships []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"relationships"`
}

type chaptersResponse struct {
	Results []mangaChapter `json:"results"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
	Total   int            `json:"total"`
}

type mangaMetadata struct {
	Result string `json:"result"`
	Data   struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title     map[string]string `json:"title"`
			AltTitles []struct {
				En string `json:"en"`
			} `json:"altTitles"`
			Description struct {
				En string `json:"en"`
			} `json:"description"`
			IsLocked bool `json:"isLocked"`
			Links    struct {
				Al    string `json:"al"`
				Ap    string `json:"ap"`
				Bw    string `json:"bw"`
				Kt    string `json:"kt"`
				Mu    string `json:"mu"`
				Amz   string `json:"amz"`
				Ebj   string `json:"ebj"`
				Mal   string `json:"mal"`
				Raw   string `json:"raw"`
				Engtl string `json:"engtl"`
			} `json:"links"`
			OriginalLanguage       string      `json:"originalLanguage"`
			LastVolume             interface{} `json:"lastVolume"`
			LastChapter            string      `json:"lastChapter"`
			PublicationDemographic string      `json:"publicationDemographic"`
			Status                 string      `json:"status"`
			Year                   interface{} `json:"year"`
			ContentRating          string      `json:"contentRating"`
			Tags                   []struct {
				ID         string `json:"id"`
				Type       string `json:"type"`
				Attributes struct {
					Name struct {
						En string `json:"en"`
					} `json:"name"`
					Version int `json:"version"`
				} `json:"attributes"`
			} `json:"tags"`
			CreatedAt time.Time   `json:"createdAt"`
			UpdatedAt interface{} `json:"updatedAt"`
			Version   int         `json:"version"`
		} `json:"attributes"`
	} `json:"data"`
	Relationships []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"relationships"`
}

const chaptersURL = "https://api.mangadex.org/manga/%s/feed?limit=%d&offset=%d&translatedLanguage[]=%s&order[chapter]=desc"

func getChapterPage(mid string, offset int) (chaptersResponse, error) {
	resp, err := client.Get(fmt.Sprintf(chaptersURL, mid, pageSize, offset, "en"))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln("Manga "+mid, resp.Request.URL, err)
		return chaptersResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Errorln("Manga "+mid, resp.Request.URL, errors.New(resp.Status), string(body))
		return chaptersResponse{}, err
	}

	var cr chaptersResponse
	err = json.Unmarshal(body, &cr)
	if err != nil {
		log.Errorln("Manga "+mid, resp.Request.URL, err, string(body))
		return chaptersResponse{}, err
	}

	return cr, nil
}

func getAllChapters(mid string) ([]mangaChapter, error) {
	total := 1
	offset := 0
	chapters := []mangaChapter{}

	for offset < total {
		<-time.After(delay)
		cr, err := getChapterPage(mid, offset)
		if err != nil {
			return nil, err
		}

		chapters = append(chapters, cr.Results...)
		total = cr.Total

		if len(cr.Results) != pageSize && offset+len(cr.Results) < total {
			log.Warningf("Manga %s: invalid chapter pagination. "+
				"Requested %d chapters at offset %d with %d total but got %d\n",
				mid, pageSize, offset, total, len(cr.Results))
		}

		offset += pageSize
	}

	return chapters, nil
}
func main() {
	mid := os.Args[1]

	<-time.After(delay)

	resp, err := client.Get(fmt.Sprintf(mangaURL, mid))
	if err != nil {
		log.Errorln("Manga "+mid, resp.Request.URL, err)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln("Manga "+mid, resp.Request.URL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Errorln("Manga "+mid, resp.Request.URL, errors.New(resp.Status), string(body))
		return
	}

	var m mangaMetadata
	err = json.Unmarshal(body, &m)
	if err != nil {
		log.Errorln("Manga "+mid, resp.Request.URL, err, string(body))
		return
	}

	if m.Result != "ok" {
		log.Errorln("Manga "+mid, resp.Request.URL, errors.New(m.Result), string(body))
		return
	}

	chapters, err := getAllChapters(mid)
	if err != nil {
		log.Errorln("Manga "+mid, "Error fetching chapters", err)
		return
	}
	log.Debugf("Fetched %d chapters for %s\n", len(chapters), mid)

	title, ok := m.Data.Attributes.Title["en"]

	// If there's no English title, pick any title at all. It doesn't matter.
	if !ok {
		for _, v := range m.Data.Attributes.Title {
			title = v
			break
		}
	}

	feed := &feeds.Rss{Feed: &feeds.Feed{
		Title: title,
		Link:  &feeds.Link{Href: "https://mangadex.org/title/" + mid},
	}}

	for _, c := range chapters {
		t := title + " - "
		if c.Data.Attributes.Volume != nil {
			t += "Volume " + (string)(*c.Data.Attributes.Volume) + ", "
		}
		t += "Chapter " + (string)(c.Data.Attributes.Chapter)

		// Old mangadex feeds used the chapter URL as the key.
		url := "https://mangadex.org/chapter/" + c.Data.ID

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   t,
			Id:      url,
			Link:    &feeds.Link{Href: url},
			Created: c.Data.Attributes.CreatedAt,
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
