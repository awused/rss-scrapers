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

const searchURLPrefix = "https://api.amiami.com/api/v1.0/items?pagemax=20&lang=eng&s_sortkey=regtimed&s_st_list_preorder_available=1&s_st_list_newitem_available=1&"

const itemURLPrefix = "https://www.amiami.com/eng/detail/?gcode="
const searchPagePrefix = "https://www.amiami.com/eng/search/list/?s_st_list_preorder_available=1&s_st_list_newitem_available=1&s_sortkey=regtimed&"

func main() {
	if len(os.Args) < 2 {
		log.Panic("Specify search parameters")
	}

	search := os.Args[1]
	searchURL := searchURLPrefix + search

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Panic(err)
	}

	req.Header.Add("X-User-Key", "amiami_dev")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Panic(err)
	}

	var b response
	err = json.Unmarshal(body, &b)
	if err != nil {
		log.Panic(err)
	}
	if !b.RSuccess {
		log.Panic("AmiAmi search failure " + b.RMessage)
	}

	feed := &feeds.Rss{&feeds.Feed{
		Title:       "AmiAmi - " + os.Args[1],
		Link:        &feeds.Link{Href: searchPagePrefix + search},
		Description: "AmiAmi - " + os.Args[1],
	}}

	now := time.Now()

	for _, p := range b.Items {
		if p.OrderClosedFlg == 1 {
			continue
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Title:   p.Gname,
			Id:      p.Gcode,
			Link:    &feeds.Link{Href: itemURLPrefix + p.Gcode},
			Created: now,
		})
	}

	rssFeed := feed.RssFeed()
	// 1 hour TTL
	rssFeed.Ttl = 60

	feedXML, err := xml.Marshal(rssFeed.FeedXml())
	if err != nil {
		log.Panic(err)
	}

	fmt.Print(string(feedXML))
}

type response struct {
	RSuccess     bool        `json:"RSuccess"`
	RValue       interface{} `json:"RValue"`
	RMessage     string      `json:"RMessage"`
	SearchResult struct {
		TotalResults int `json:"total_results"`
	} `json:"search_result"`
	Items []struct {
		Gcode                  string      `json:"gcode"`
		Gname                  string      `json:"gname"`
		ThumbURL               string      `json:"thumb_url"`
		ThumbAlt               string      `json:"thumb_alt"`
		ThumbTitle             string      `json:"thumb_title"`
		MinPrice               int         `json:"min_price"`
		MaxPrice               int         `json:"max_price"`
		CPriceTaxed            int         `json:"c_price_taxed"`
		MakerName              string      `json:"maker_name"`
		Saleitem               int         `json:"saleitem"`
		ConditionFlg           int         `json:"condition_flg"`
		ListPreorderAvailable  int         `json:"list_preorder_available"`
		ListBackorderAvailable int         `json:"list_backorder_available"`
		ListStoreBonus         int         `json:"list_store_bonus"`
		ListAmiamiLimited      int         `json:"list_amiami_limited"`
		InstockFlg             int         `json:"instock_flg"`
		OrderClosedFlg         int         `json:"order_closed_flg"`
		ElementID              interface{} `json:"element_id"`
		Salestatus             string      `json:"salestatus"`
		SalestatusDetail       string      `json:"salestatus_detail"`
		Releasedate            string      `json:"releasedate"`
		Jancode                string      `json:"jancode"`
		Preorderitem           int         `json:"preorderitem"`
		Saletopitem            int         `json:"saletopitem"`
		ResaleFlg              int         `json:"resale_flg"`
		PreownedSaleFlg        interface{} `json:"preowned_sale_flg"`
		ForWomenFlg            int         `json:"for_women_flg"`
		GenreMoe               int         `json:"genre_moe"`
		Cate6                  interface{} `json:"cate6"`
		Cate7                  interface{} `json:"cate7"`
		BuyFlg                 int         `json:"buy_flg"`
		BuyPrice               int         `json:"buy_price"`
		BuyRemarks             interface{} `json:"buy_remarks"`
		StockFlg               int         `json:"stock_flg"`
		ImageOn                int         `json:"image_on"`
		ImageCategory          string      `json:"image_category"`
		ImageName              string      `json:"image_name"`
		Metaalt                interface{} `json:"metaalt"`
	} `json:"items"`
	Embedded struct {
		CategoryTags []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"category_tags"`
		Makers []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"makers"`
		SeriesTitles []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"series_titles"`
		OriginalTitles []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"original_titles"`
		CharacterNames []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"character_names"`
		Elements []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"elements"`
	} `json:"_embedded"`
}
