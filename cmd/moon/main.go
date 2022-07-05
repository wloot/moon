package main

import (
	"bytes"
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
	"moon/pkg/subtype"
	"moon/pkg/unpack"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
)

type Subinfo struct {
	format  string
	data    []byte
	info    *astisub.Subtitles
	chinese bool
	double  bool
	tc      bool
}

var SETTNGS_videopath_map map[string]string = map[string]string{}

var SETTINGS_emby_url string = "http://play.charontv.com"
var SETTINGS_emby_key string = "fe1a0f6c143043e98a1f3099bfe0a3a8"
var SETTINGS_emby_importcount int = 200

func main() {
start:
	embyAPI := emby.New(SETTINGS_emby_url, SETTINGS_emby_key)
	zimukuAPI := zimuku.New()

	var videoList []emby.EmbyVideo
	for i := 0; len(videoList) < SETTINGS_emby_importcount; i += 1 {
		ids := embyAPI.RecentVideo(SETTINGS_emby_importcount/2, i*SETTINGS_emby_importcount/2, "Movie,Season")
		for _, id := range ids {
			v := embyAPI.ItemInfo(id)
			if v.Type == "Movie" {
				if len(v.ProductionLocations) > 0 && v.ProductionLocations[0] == "China" {
					continue
				}
				if v.MediaStreams[1].Type == "Audio" && v.MediaStreams[1].DisplayLanguage == "Chinese Simplified" {
					continue
				}
			} else if v.Type == "Season" {
				if strings.HasPrefix(v.Path, "/gd/国产剧/") {
					continue
				}
			}
			originalTitle := v.OriginalTitle
			if v.Type == "Season" {
				s := embyAPI.ItemInfo(v.SeriesId)
				originalTitle = s.OriginalTitle
			}
			if whatlanggo.Detect(originalTitle).Lang == whatlanggo.Cmn {
				continue
			}
			videoList = append(videoList, v)
		}
	}

	for _, v := range videoList {
		if v.Type == "Season" {
			season := v
			series := embyAPI.ItemInfo(v.SeriesId)
			episodes := embyAPI.Episodes(v.SeriesId, v.Id)

			for i, episode := range episodes {
				if episode.IndexNumber == 1 {
					if episode.ProviderIds.Imdb == "" {
						embyAPI.Refresh(episode.Id, true)
						time.Sleep(30 * time.Second)
						episodes[i] = embyAPI.ItemInfo(episode.Id)
					}
				}
			}
			if series.OriginalTitle == series.Name {
				embyAPI.Refresh(series.Id, true)
				time.Sleep(30 * time.Second)
				series = embyAPI.ItemInfo(series.Id)
			}
			keywords := zimukuAPI.SeasonKeywords(season, series, episodes)

			for i := len(episodes) - 1; i >= 0; i-- {
				v := episodes[i]
				var hasSub = false
				for _, stream := range v.MediaStreams {
					if stream.Type == "Subtitle" && stream.DisplayLanguage == "Chinese Simplified" {
						if stream.IsExternal == false {
							hasSub = true
							break
						}
						path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
						// Emby 自带的字幕下载
						if strings.HasSuffix(path, ".zh-CN") == false {
							hasSub = true
							break
						}
					}
				}
				interval := time.Hour * 24 * 7
				if hasSub == true {
					interval = time.Hour * 24 * 30
					if time.Now().Sub(v.GetPremiereDate()) > 180*time.Hour*24 {
						interval = time.Hour * 24 * 180
					}
				}
				if time.Now().Sub(v.GetDateCreated()) <= time.Hour*24*3 || time.Now().Sub(v.GetPremiereDate()) <= time.Hour*24*30 {
					interval = time.Hour * 24
				}
				if ok := cache.StatKey(interval, v.Path); !ok {
					episodes = append(episodes[:i], episodes[i+1:]...)
				}
			}

			subFilesEP := zimukuAPI.SearchSeason(keywords, episodes)
			for i, subFiles := range subFilesEP {
				v := episodes[i]
				succ := writeSub(subFiles, v)
				if succ == true {
					cache.UpdateKey(v.Path)
					embyAPI.Refresh(v.Id, false)
				}
			}
			continue
		}

		var hasSub = false
		for _, stream := range v.MediaStreams {
			if stream.Type == "Subtitle" && stream.DisplayLanguage == "Chinese Simplified" {
				if stream.IsExternal == false {
					hasSub = true
					break
				}
				path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
				// Emby 自带的字幕下载
				if strings.HasSuffix(path, ".zh-CN") == false {
					hasSub = true
					break
				}
			}
		}
		interval := time.Hour * 24 * 7
		if hasSub == true {
			interval = time.Hour * 24 * 30
			if time.Now().Sub(v.GetPremiereDate()) > 180*time.Hour*24 {
				interval = time.Hour * 24 * 180
			}
		}
		if time.Now().Sub(v.GetDateCreated()) <= time.Hour*24*3 || time.Now().Sub(v.GetPremiereDate()) <= time.Hour*24*30 {
			interval = time.Hour * 24
		}
		if ok := cache.StatKey(interval, v.Path); !ok {
			continue
		}

		if v.OriginalTitle == v.Name {
			embyAPI.Refresh(v.Id, true)
			time.Sleep(30 * time.Second)
			v = embyAPI.ItemInfo(v.Id)
		}
		for old, new := range SETTNGS_videopath_map {
			if strings.HasPrefix(v.Path, old) {
				v.Path = new + v.Path[len(old):]
			}
		}

		subFiles, failed := zimukuAPI.SearchMovie(v)
		if failed == false {
			cache.UpdateKey(v.Path)
		}
		succ := writeSub(subFiles, v)
		if succ == true {
			embyAPI.Refresh(v.Id, false)
		}
	}
	fmt.Printf("all work done, sleep 6 hours")
	time.Sleep(6 * time.Hour)
	goto start
}

func writeSub(subFiles []string, v emby.EmbyVideo) bool {
	var subSorted []Subinfo
	for _, subName := range subFiles {
		err := unpack.WalkUnpacked(subName, func(reader io.Reader, info fs.FileInfo) {
			name := info.Name()
			if v.Type == "Episode" {
				if filepath.Base(name) != filepath.Base(subName) {
					ep := episode.NameToEpisode(name)
					if ep > 0 && ep != v.IndexNumber {
						return
					}
				}
			}
			t := strings.ToLower(filepath.Ext(name))
			if len(t) > 0 {
				t = t[1:]
			}
			data, _ := io.ReadAll(reader)
			if transformed, err := charset.AnyToUTF8(data); err == nil {
				data = transformed
			}
			if len(data) == 0 {
				fmt.Printf("ignoring empty sub %v\n", name)
				return
			}

			readSub := func(data []byte, ext string) (*astisub.Subtitles, error) {
				var s *astisub.Subtitles
				var err error
				if ext == "ssa" || ext == "ass" {
					// 一个常见的字幕typo
					data = bytes.Replace(data, []byte(",&H00H202020,"), []byte(",&H00202020,"), 1)
					data = bytes.Replace(data, []byte("[Aegisub Project Garbage]"), []byte(""), 1)
					s, err = astisub.ReadFromSSA(bytes.NewReader(data))
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
			subSorted = append(subSorted, Subinfo{
				data:   data,
				info:   s,
				format: t,
			})
		})
		if err != nil {
			fmt.Printf("open sub file %v faild: %v\n", subName, err)
		}
	}

	jianfan := charset.NewJianfan()
	for i := range subSorted {
		countTC := 0
		countChars := 0
		countCh := 0
		countLines := 0
		countAllLines := 0
		durationLast := make(map[string]struct{})
		for _, v := range subSorted[i].info.Items {
			if len(v.Lines) == 0 {
				continue
			}
			countAllLines += len(v.Lines)

			key := v.StartAt.String() + "-" + v.EndAt.String()
			if _, ok := durationLast[key]; ok == true {
				continue
			}
			durationLast[key] = struct{}{}

			line := v.Lines[0].String()
			//`<(.+)( .*)?>([\s\S]*?)<\/(\1)>`
			line = regexp.MustCompile(`(?m)((?i){[^}]*})`).ReplaceAllString(line, "")
			line = regexp.MustCompile(`(?m)((?i)\<[^>]*\>)`).ReplaceAllString(line, "")
			line = strings.ReplaceAll(line, `\n`, `\N`)
			line = strings.Split(line, `\N`)[0]

			countLines += 1
			lang := whatlanggo.Detect(line)
			if lang.Lang == whatlanggo.Cmn {
				countCh += 1

				countChars += len([]rune(line))
				countTC += jianfan.CountCht(line)
			}
		}

		if countLines/2 < countCh {
			subSorted[i].chinese = true
		}
		if countLines*3 < countAllLines*2 {
			subSorted[i].double = true
		}
		if countChars/10 <= countTC {
			subSorted[i].tc = true
		}
	}
	for i := len(subSorted) - 1; i >= 0; i-- {
		need := true
		if subSorted[i].chinese == false {
			need = false
		}
		if subSorted[i].tc == true {
			need = false
		}
		if need == false {
			subSorted = append(subSorted[:i], subSorted[i+1:]...)
		}
	}

	if len(subSorted) == 0 {
		fmt.Printf("total sub downloaded is 0\n")
		return false
	}
	sort.Slice(subSorted, func(i, j int) bool {
		if subSorted[i].double != subSorted[j].double {
			return subSorted[i].double == true
		}
		return false
	})

	selectedSub := subSorted[0]
	backupType := "srt"
	if selectedSub.format == "srt" {
		backupType = "ass"
	}
	var reference string
savesub:
	name := v.Path[:len(v.Path)-len(filepath.Ext(v.Path))] + ".chs." + selectedSub.format
	err := os.WriteFile(name, selectedSub.data, 0644)
	if err != nil {
		fmt.Printf("failed to write sub file: %v\n", err)
		if backupType == "" {
			return true
		}
		return false
	}
	fmt.Printf("sub written to %v\n", name)
	if reference == "" && backupType != "" {
		reference = cache.TryGet(v.Path, func() string {
			reference := ffsubsync.FindBestReferenceSub(v)
			if reference == "" {
				fmt.Printf("no fit inter sub so extract audio for sync\n")
				reference, _ = ffmpeg.KeepAudio(v.Path)
			}
			return reference
		})
	}
	if reference == "" {
		ffsubsync.DoSync(name, v.Path, false)
	} else {
		ffsubsync.DoSync(name, reference, true)
	}
	if backupType != "" {
		for i := range subSorted {
			if subSorted[i].format == backupType {
				backupType = ""
				selectedSub = subSorted[i]
				goto savesub
			}
		}
	}
	return true
}
