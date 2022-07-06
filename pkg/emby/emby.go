package emby

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Emby struct {
	url    string
	key    string
	client *http.Client
}

type videoList struct {
	Items []struct {
		Id string `json:"Id"`
	} `json:"Items"`
}

type episodeList struct {
	Items []EmbyVideo `json:"Items"`
}

type EmbyVideoStream struct {
	Codec           string `json:"Codec"`
	Language        string `json:"Language"`
	DisplayLanguage string `json:"DisplayLanguage"`
	Type            string `json:"Type"`
	Title           string `json:"Title"`
	IsExternal      bool   `json:"IsExternal"`
	Index           int    `json:"Index"`
	Path            string `json:"Path"`
}

func (e EmbyVideoStream) SubtitleCodecToFfmpeg() string {
	if e.Codec == "PGSSUB" {
		return "hdmv_pgs_subtitle"
	}
	if e.Codec == "DVDSUB" {
		return "dvd_subtitle"
	}
	return e.Codec
}

type EmbyVideo struct {
	Id            string `json:"Id"`
	Name          string `json:"Name"`
	OriginalTitle string `json:"OriginalTitle"`
	Path          string `json:"Path"`
	ProviderIds   struct {
		Tmdb string `json:"Tmdb"`
		Imdb string `json:"Imdb"`
	} `json:"ProviderIds"`
	ProductionYear      int               `json:"ProductionYear"`
	MediaStreams        []EmbyVideoStream `json:"MediaStreams"`
	ProductionLocations []string          `json:"ProductionLocations"`
	DateCreated         string            `json:"DateCreated"`
	PremiereDate        string            `json:"PremiereDate"`
	Type                string            `json:"Type"`
	SeriesId            string            `json:"SeriesId"`
	IndexNumber         int               `json:"IndexNumber"`
	IndexNumberEnd      int               `json:"IndexNumberEnd"`
}

func (e EmbyVideo) parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.0000000Z", s)
	return t
}

func (e EmbyVideo) GetDateCreated() time.Time {
	return e.parseTime(e.DateCreated)
}

func (e EmbyVideo) GetPremiereDate() time.Time {
	return e.parseTime(e.PremiereDate)
}

func New(url string, key string) *Emby {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	//tr.MaxIdleConnsPerHost = tr.MaxIdleConns
	//tr.ForceAttemptHTTP2 = false
	return &Emby{
		url: url,
		key: key,
		client: &http.Client{
			Transport: tr,
		},
	}
}

func (e *Emby) buildURL(path string) string {
	delimiter := "?"
	if strings.Contains(path, delimiter) {
		delimiter = "&"
	}
	return e.url + path + delimiter + "api_key=" + e.key
}

func (e *Emby) getJson(url string, v interface{}) error {
	resp, err := e.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return err
	}
	return nil
}

func (e *Emby) ItemInfo(id string) EmbyVideo {
	var info EmbyVideo
	e.getJson(e.buildURL("/LiveTv/Programs/"+id), &info)

	return info
}

// 返回的信息是不完整的
func (e *Emby) Episodes(seriesId string, seasonId string) []EmbyVideo {
	var list episodeList
	e.getJson(e.buildURL("/Shows/"+seriesId+"/Episodes?SeasonId="+seasonId), &list)

	return list.Items
}

func (e *Emby) RecentItems(num int, start int, types string) []string {
	var list videoList
	e.getJson(e.buildURL(
		"/Items?Limit="+strconv.Itoa(num)+"&IncludeItemTypes="+types+"&SortBy=DateCreated&SortOrder=Descending&Recursive=true&StartIndex="+strconv.Itoa(start),
	), &list)

	var result []string
	for _, v := range list.Items {
		result = append(result, v.Id)
	}
	return result
}

func (e *Emby) Refresh(id string, replace bool) {
	url := "/Items/" + id + "/Refresh?Recursive=false&ImageRefreshMode=Default&ReplaceAllImages=false"
	if replace == true {
		url += "&MetadataRefreshMode=FullRefresh&ReplaceAllMetadata=true"
	} else {
		url += "&MetadataRefreshMode=Default&ReplaceAllMetadata=false"
	}
	url = e.buildURL(url)
	resp, err := e.client.Post(url, "", nil)
	if err != nil {
		return
	}
	resp.Body.Close()
}
