package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"moon/pkg/cache"
	"moon/pkg/charset"
	"moon/pkg/emby"
	"moon/pkg/episode"
	"moon/pkg/ffmpeg"
	"moon/pkg/ffsubsync"
	"moon/pkg/provider/zimuku"
	"moon/pkg/subtitle"
	"moon/pkg/subtype"
	"moon/pkg/unpack"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
)

type subinfo struct {
	format  string
	name    string
	data    []byte
	info    *astisub.Subtitles
	analyze subtitle.SubContent
}

var SETTNGS_videopath_map map[string]string = map[string]string{}

var SETTINGS_emby_url string = "http://172.16.238.10:8096"
var SETTINGS_emby_key string = "fe1a0f6c143043e98a1f3099bfe0a3a8"
var SETTINGS_emby_importcount int = 200

func main() {
start:
	embyAPI := emby.New(SETTINGS_emby_url, SETTINGS_emby_key)
	zimukuAPI := zimuku.New()

	var firstTime time.Time
	failedTimes := 0
	processedItems := 0
	loopCount := -1

start_continue:
	loopCount += 1
	var itemList []emby.EmbyVideo
	items := embyAPI.RecentItems(SETTINGS_emby_importcount, SETTINGS_emby_importcount*loopCount, "Movie,Episode")
	if loopCount == 0 {
		firstTime = items[0].GetDateCreated()
	}
	itemList = filterItems(embyAPI, items)
	if loopCount > 0 {
		newItems := embyAPI.RecentItems(SETTINGS_emby_importcount/2, 0, "Movie,Episode")
		for i := len(newItems) - 1; i >= 0; i-- {
			if newItems[i].GetDateCreated().Sub(firstTime) <= 0 {
				newItems = append(newItems[:i], newItems[i+1:]...)
				continue
			}
			firstTime = newItems[0].GetDateCreated()
			itemList = append(filterItems(embyAPI, newItems), itemList...)
			break
		}
	}
	if len(items) == 0 && len(itemList) == 0 {
		fmt.Printf("no jobs to run after proessing %v items, sleep\n", processedItems)
		zimukuAPI.Close()
		time.Sleep(24 * time.Hour)
		goto start
	}

	for _, v := range itemList {
		if failedTimes >= 5 {
			fmt.Printf("too much errors after proessing %v items, sleep\n", processedItems)
			zimukuAPI.Close()
			time.Sleep(3 * time.Hour)
			goto start
		}

		var processed, failed bool
		if v.Type == "Season" {
			processed, failed = season(v, embyAPI, zimukuAPI)
		}
		if v.Type == "Movie" {
			processed, failed = movie(v, embyAPI, zimukuAPI)
		}
		if processed {
			processedItems += 1
			if failed {
				failedTimes += 1
			} else {
				failedTimes = 0
			}
		}
	}
	goto start_continue
}

func season(v emby.EmbyVideo, embyAPI *emby.Emby, zimukuAPI *zimuku.Zimuku) (processed bool, failed bool) {
	season := v
	series := embyAPI.ItemInfo(v.SeriesId)
	episodes := embyAPI.Episodes(v.SeriesId, v.Id)

	// 暂不支持单V多E
	if len(episodes) == 0 || episodes[0].IndexNumberEnd != 0 {
		return
	}
	var epOne emby.EmbyVideo
	for i := range episodes {
		// 获取完整信息
		episodes[i] = embyAPI.ItemInfo(episodes[i].Id)
		if episodes[i].IndexNumber == 1 {
			epOne = episodes[i]
		}
	}

	for i := len(episodes) - 1; i >= 0; i-- {
		v := episodes[i]
		if v.IndexNumber <= 0 {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
		if len(v.MediaStreams) <= 1 || (v.MediaStreams[1].Type == "Audio" && v.MediaStreams[1].DisplayLanguage == "Chinese Simplified") {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
		var hasExtSub = false
		var hasIntSub = false
		for _, stream := range v.MediaStreams {
			if stream.Type == "Subtitle" && stream.DisplayLanguage == "Chinese Simplified" {
				if stream.IsExternal == false {
					if stream.Codec == "PGSSUB" || stream.Codec == "DVDSUB" {
						continue
					}
					hasIntSub = true
				}
				path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
				// Emby 自带的字幕下载
				if strings.HasSuffix(path, ".zh-CN") == false {
					hasExtSub = true
				}
			}
		}
		if hasIntSub {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
		if v.Path == "" {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
		var interval time.Duration
		if hasExtSub == true {
			interval = time.Hour * 24 * 30
			if time.Now().Sub(v.GetPremiereDate()) > time.Hour*24*180 {
				interval = time.Hour * 24 * 90
			}
		} else {
			interval = time.Hour * 24 * 14
			if time.Now().Sub(v.GetPremiereDate()) > time.Hour*24*180 {
				interval = time.Hour * 24 * 60
			}
		}
		if time.Now().Sub(v.GetPremiereDate()) < time.Hour*24*7 && time.Now().Sub(v.GetDateCreated()) < time.Hour*24*7 {
			interval = time.Hour * 24
		}
		if ok := cache.StatKey(interval, v.MediaSources[0].ID, "videos"); !ok {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
		if _, err := os.Stat(v.Path); errors.Is(err, os.ErrNotExist) {
			episodes = append(episodes[:i], episodes[i+1:]...)
			continue
		}
	}
	if len(episodes) == 0 {
		return
	}

	if epOne.ProviderIds.Imdb == "" && season.IndexNumber != 1 {
		embyAPI.Refresh(epOne.Id, true)
		time.Sleep(20 * time.Second)
		epOne = embyAPI.ItemInfo(epOne.Id)
	}
	if series.OriginalTitle == series.Name || (series.ProviderIds.Imdb == "" && season.IndexNumber == 1) {
		embyAPI.Refresh(series.Id, true)
		time.Sleep(20 * time.Second)
		series = embyAPI.ItemInfo(series.Id)
	}
	keywords := zimukuAPI.SeasonKeywords(season, series, []emby.EmbyVideo{epOne})
	if len(keywords) == 0 {
		return
	}

	processed = true
	subFilesEP := zimukuAPI.SearchSeason(keywords, episodes)
	for i, subFiles := range subFilesEP {
		v := episodes[i]
		for old, new := range SETTNGS_videopath_map {
			if strings.HasPrefix(v.Path, old) {
				v.Path = new + v.Path[len(old):]
			}
		}
		if len(subFiles) > 0 {
			succ, err := writeSub(subFiles, v)
			if err == nil {
				cache.UpdateKey(v.MediaSources[0].ID, "videos")
			}
			if succ == true {
				embyAPI.Refresh(v.Id, false)
			}
		} else {
			cache.UpdateKey(v.MediaSources[0].ID, "videos")
		}
	}
	if len(subFilesEP) != len(episodes) {
		failed = true
	}
	return
}

func movie(v emby.EmbyVideo, embyAPI *emby.Emby, zimukuAPI *zimuku.Zimuku) (processed bool, failed bool) {
	var hasExtSub = false
	var hasIntSub = false
	for _, stream := range v.MediaStreams {
		if stream.Type == "Subtitle" && stream.DisplayLanguage == "Chinese Simplified" {
			if stream.IsExternal == false {
				if stream.Codec == "PGSSUB" || stream.Codec == "DVDSUB" {
					continue
				}
				hasIntSub = true
			}
			path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
			// Emby 自带的字幕下载
			if strings.HasSuffix(path, ".zh-CN") == false {
				hasExtSub = true
			}
		}
	}
	if hasIntSub {
		return
	}
	var interval time.Duration
	if hasExtSub == true {
		interval = time.Hour * 24 * 14
		if time.Now().Sub(v.GetPremiereDate()) > time.Hour*24*360 && time.Now().Sub(v.GetDateCreated()) > time.Hour*24*30 {
			interval = time.Hour * 24 * 90
		}
	} else {
		interval = time.Hour * 24 * 7
		if time.Now().Sub(v.GetPremiereDate()) > time.Hour*24*360 && time.Now().Sub(v.GetDateCreated()) > time.Hour*24*30 {
			interval = time.Hour * 24 * 60
		}
	}
	if time.Now().Sub(v.GetPremiereDate()) < time.Hour*24*270 && time.Now().Sub(v.GetDateCreated()) < time.Hour*24*14 {
		interval = time.Hour * 24
	}

	if ok := cache.StatKey(interval, v.MediaSources[0].ID, "videos"); !ok {
		return
	}

	if _, err := os.Stat(v.Path); errors.Is(err, os.ErrNotExist) {
		return
	}

	if v.OriginalTitle == v.Name {
		embyAPI.Refresh(v.Id, true)
		time.Sleep(20 * time.Second)
		v = embyAPI.ItemInfo(v.Id)
	}
	for old, new := range SETTNGS_videopath_map {
		if strings.HasPrefix(v.Path, old) {
			v.Path = new + v.Path[len(old):]
		}
	}

	processed = true
	var subFiles []string
	subFiles, failed = zimukuAPI.SearchMovie(v)
	if failed == true || len(subFiles) == 0 {
		if failed == false {
			cache.UpdateKey(v.MediaSources[0].ID, "videos")
		}
		return
	}
	succ, err := writeSub(subFiles, v)
	if err == nil {
		cache.UpdateKey(v.MediaSources[0].ID, "videos")
	}
	if succ == true {
		embyAPI.Refresh(v.Id, false)
	}
	return
}

func writeSub(subFiles []string, v emby.EmbyVideo) (bool, error) {
	var subSorted []subinfo
	for _, subName := range subFiles {
		err := unpack.WalkUnpacked(subName, func(reader io.Reader, info fs.FileInfo) {
			name := info.Name()
			if v.Type == "Episode" {
				if filepath.Base(name) != filepath.Base(subName) {
					ep := episode.NameToEpisode(name)
					if ep <= 0 || ep != v.IndexNumber {
						fmt.Printf("skip file %v as ep number not match %v\n", name, v.IndexNumber)
						return
					}
					se := episode.NameToSeason(name)
					if se >= 0 && v.ParentIndexNumber != se {
						fmt.Printf("skip file %v as se number not match %v\n", name, v.ParentIndexNumber)
						return
					}
				}
			}
			t := strings.ToLower(filepath.Ext(name))
			if len(t) > 0 {
				t = t[1:]
			}
			var data = make([]byte, 0, info.Size())
			_, err := reader.Read(data)
			if err == nil {
				if transformed, err := charset.AnyToUTF8(data); err == nil {
					data = transformed
				}
				data = charset.RemoveBom(data)
			}
			if len(data) == 0 || len(data) != cap(data) {
				fmt.Printf("ignoring empty/size-mismatched sub %v\n", name)
				return
			}

			readSub := func(data []byte, ext string) (*astisub.Subtitles, error) {
				var s *astisub.Subtitles
				var err error
				if ext == "ssa" || ext == "ass" {
					// 一个常见的字幕typo
					data = bytes.Replace(data, []byte(",&H00H202020,"), []byte(",&H00202020,"), 1)
					s, err = astisub.ReadFromSSAWithOptions(bytes.NewReader(data), astisub.SSAOptions{})
				}
				if ext == "srt" {
					s, err = astisub.ReadFromSRT(bytes.NewReader(data))
				}
				if ext == "vtt" {
					s, err = astisub.ReadFromWebVTT(bytes.NewReader(data))
				}
				return s, err
			}
			s, err := readSub(data, t)
			if s == nil || err != nil || len(s.Items) == 0 {
				t = subtype.GuessingType(string(data))
				s, err = readSub(data, t)
				if err != nil || s == nil || len(s.Items) == 0 {
					fmt.Printf("ignoring sub %v as err %v or guessed type '%v'\n", name, err, t)
					return
				}
			}

			if t == "vtt" {
				var buf bytes.Buffer
				s.WriteToSRT(&buf)
				data = buf.Bytes()
				t = "srt"
			}
			subSorted = append(subSorted, subinfo{
				data:   data,
				info:   s,
				format: t,
				name:   name,
			})
		})
		if err != nil {
			fmt.Printf("open sub file %v faild: %v\n", subName, err)
		}
	}

	for i := range subSorted {
		if subSorted[i].format == "srt" {
			subSorted[i].analyze = subtitle.AnalyzeSRT(subSorted[i].info)
		}
		if subSorted[i].format == "ass" || subSorted[i].format == "ssa" {
			subSorted[i].analyze = subtitle.AnalyzeASS(subSorted[i].info)
		}
	}
	for i := len(subSorted) - 1; i >= 0; i-- {
		need := true
		if subSorted[i].analyze.Chinese == false {
			fmt.Printf("skip sub %v as not chinese\n", subSorted[i].name)
			need = false
		}
		if need == true && subSorted[i].analyze.Cht == true {
			fmt.Printf("skip sub %v as its cht\n", subSorted[i].name)
			need = false
		}
		if need == false {
			subSorted = append(subSorted[:i], subSorted[i+1:]...)
		}
	}
	if len(subSorted) == 0 {
		fmt.Printf("total sub downloaded is 0\n")
		return false, nil
	}

	sort.Slice(subSorted, func(i, j int) bool {
		if subSorted[i].analyze.OriFirst != subSorted[j].analyze.OriFirst {
			return subSorted[i].analyze.OriFirst == false
		}
		if subSorted[i].analyze.Double != subSorted[j].analyze.Double {
			return subSorted[i].analyze.Double == true
		}
		if subSorted[i].format != subSorted[j].format {
			return subSorted[i].format == "ass"
		}
		return false
	})

	name := v.Path[:len(v.Path)-len(filepath.Ext(v.Path))] + ".chs." + subSorted[0].format
	err := os.WriteFile(name, subSorted[0].data, 0644)
	if err != nil {
		fmt.Printf("failed to write sub file: %v\n", err)
		return false, err
	}
	fmt.Printf("sub %v written to %v\n", subSorted[0].name, name)
	reference := cache.TryGet(v.MediaSources[0].ID, "references", func() string {
		reference := ffsubsync.FindBestReferenceSub(v)
		if reference == "" {
			fmt.Printf("no fit inter sub so extract audio for sync\n")
			reference, _ = ffmpeg.KeepAudio(v.Path)
		}
		return reference
	})
	if reference == "" {
		ffsubsync.Sync(name, v.Path, false)
	} else {
		ffsubsync.Sync(name, reference, true)
	}
	return true, nil
}

func filterItems(embyAPI *emby.Emby, items []emby.EmbyItem) []emby.EmbyVideo {
	var itemList []emby.EmbyVideo
	for _, item := range items {
		v := embyAPI.ItemInfo(item.Id)
		if v.Type == "Movie" {
			if v.Path == "" {
				continue
			}
			if len(v.ProductionLocations) > 0 && v.ProductionLocations[0] == "China" {
				continue
			}
			if len(v.MediaStreams) <= 1 || (v.MediaStreams[1].Type == "Audio" && v.MediaStreams[1].DisplayLanguage == "Chinese Simplified") {
				continue
			}
			if whatlanggo.Detect(v.OriginalTitle).Lang == whatlanggo.Cmn {
				continue
			}
			itemList = append(itemList, v)
		} else if v.Type == "Episode" {
			if v.ParentIndexNumber <= 0 {
				continue
			}
			if v.Path == "" || strings.HasPrefix(v.Path, "/gd/国产剧/") {
				continue
			}
			need := true
			for i := range itemList {
				if itemList[i].Type == "Season" && itemList[i].Id == v.SeasonId {
					need = false
					break
				}
			}
			if need == false {
				continue
			}
			series := embyAPI.ItemInfo(v.SeriesId)
			if whatlanggo.Detect(series.OriginalTitle).Lang == whatlanggo.Cmn {
				continue
			}
			season := embyAPI.ItemInfo(v.SeasonId)
			itemList = append(itemList, season)
		}
	}
	return itemList
}
