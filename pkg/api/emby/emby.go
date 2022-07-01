package emby

import (
	"encoding/json"
	"io/ioutil"
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

var SubtitleCodecToFormat map[string]string = map[string]string{
	"pgssub": "sup", // upper
	"subrip": "srt",
	"ass":    "ass",
	//"mov_text": "jacosub",
	//"dvdsub": "microdvd", // upper

	"ssa": "ass",
	"srt": "srt",
	"vtt": "srt",
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

func (e *Emby) MovieInfo(id string) EmbyVideo {
	var info EmbyVideo
	e.getJson(e.buildURL("/LiveTv/Programs/"+id), &info)

	return info
}

func (e *Emby) RecentMovie(num int, start int) []string {
	var list videoList
	e.getJson(e.buildURL(
		"/Items?Limit="+strconv.Itoa(num)+"&IncludeItemTypes=Movie&SortBy=DateCreated&SortOrder=Descending&Recursive=true&StartIndex="+strconv.Itoa(start),
	), &list)

	var result []string
	for _, v := range list.Items {
		result = append(result, v.Id)
	}
	return result
}

func (e *Emby) Refresh(id string, replace bool) {
	url := "/Items/" + id + "/Refresh?Recursive=true&ImageRefreshMode=Default&ReplaceAllImages=false"
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
