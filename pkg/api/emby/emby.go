package emby

import (
	"encoding/json"
	"io/ioutil"
	"moon/pkg/video"
	"net/http"
	"strconv"
	"strings"
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

type videoInfo struct {
	Name          string `json:"Name"`
	OriginalTitle string `json:"OriginalTitle"`
	Path          string `json:"Path"`
	ProviderIds   struct {
		Tmdb string `json:"Tmdb"`
		Imdb string `json:"Imdb"`
	} `json:"ProviderIds"`
	ProductionYear int `json:"ProductionYear"`
}

func New(url string, key string) *Emby {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = tr.MaxIdleConns
	tr.ForceAttemptHTTP2 = false
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

func (e *Emby) MovieInfo(id string) video.Movie {
	var info videoInfo
	e.getJson(e.buildURL("/LiveTv/Programs/"+id), &info)

	video := video.Movie{
		EmbyId: id,
		TmdbId: info.ProviderIds.Tmdb,
		ImdbId: info.ProviderIds.Imdb,
		Path:   info.Path,
		Year:   info.ProductionYear,
	}
	video.Titles = []string{info.OriginalTitle}
	if info.OriginalTitle != info.Name {
		video.Titles = append(video.Titles, info.Name)
	}

	return video
}

func (e *Emby) RecentMovie(num int) []string {
	var list videoList
	e.getJson(e.buildURL("/Items?Limit="+strconv.Itoa(num)+"&IncludeItemTypes=Movie&SortBy=DateCreated&SortOrder=Descending&Recursive=true"), &list)

	var result []string
	for _, v := range list.Items {
		result = append(result, v.Id)
	}
	return result
}

func (e *Emby) Refresh(id string, replace bool) {
	url := "/Items/" + id + "/Refresh?Recursive=true&ImageRefreshMode=Default&ReplaceAllImages=false&ReplaceAllMetadata=false"
	if replace == true {
		url += "&MetadataRefreshMode=FullRefresh&ReplaceAllMetadata=true"
	} else {
		url += "&MetadataRefreshMode=Default&ReplaceAllMetadata=false"
	}
	url = e.buildURL(url)
	resp, err := e.client.Get(url)
	if err != nil {
		return
	}
	resp.Body.Close()
}
