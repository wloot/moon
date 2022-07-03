package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"moon/pkg/charset"
	"moon/pkg/emby"
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
	zimuku := zimuku.New()

	var movieList []emby.EmbyVideo
	for i := 0; len(movieList) < SETTINGS_emby_importcount; i += 1 {
		ids := embyAPI.RecentMovie(SETTINGS_emby_importcount/2, i*SETTINGS_emby_importcount/2)
		for _, id := range ids {
			v := embyAPI.MovieInfo(id)
			if len(v.ProductionLocations) > 0 && v.ProductionLocations[0] == "China" {
				continue
			}
			if whatlanggo.Detect(v.OriginalTitle).Lang == whatlanggo.Cmn {
				continue
			}
			need := true
			for _, stream := range v.MediaStreams {
				if stream.Index == 1 && stream.Type == "Audio" {
					if stream.Language == "chs" || (stream.Language == "chi" && stream.DisplayLanguage == "Chinese Simplified") {
						need = false
						break
					}
				}
				if stream.Type == "Subtitle" &&
					(stream.Language == "chs" || (stream.Language == "chi" && stream.DisplayLanguage == "Chinese Simplified")) {
					if stream.IsExternal == false {
						need = false
						break
					}
					path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
					// Emby 自带的字幕下载
					if strings.HasSuffix(path, ".zh-CN") == false {
						need = false
						break
					}
				}
			}
			if need == true {
				movieList = append(movieList, v)
			}
		}
	}

	for _, v := range movieList {
		if v.OriginalTitle == v.Name {
			embyAPI.Refresh(v.Id, true)
			time.Sleep(30 * time.Second)
			v = embyAPI.MovieInfo(v.Id)
		}
		for old, new := range SETTNGS_videopath_map {
			if strings.HasPrefix(v.Path, old) {
				v.Path = new + v.Path[len(old):]
			}
		}

		subFiles := zimuku.SearchMovie(v)
		var subSorted []Subinfo
		for _, subName := range subFiles {
			err := unpack.WalkUnpacked(subName, func(reader io.Reader, info fs.FileInfo) {
				name := info.Name()
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
		if len(subSorted) == 0 {
			fmt.Printf("total sub downloaded is 0\n")
			continue
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

		sort.Slice(subSorted, func(i, j int) bool {
			if subSorted[i].chinese != subSorted[j].chinese {
				return subSorted[i].chinese == true
			}
			if subSorted[i].tc != subSorted[j].tc {
				return subSorted[i].tc == false
			}
			if subSorted[i].double != subSorted[j].double {
				return subSorted[i].double == true
			}
			return false
		})

		backupType := "srt"
		if subSorted[0].format == "srt" {
			backupType = "ass"
		}
		var reference string
		selectedSub := subSorted[0]
	savesub:
		name := v.Path[:len(v.Path)-len(filepath.Ext(v.Path))] + ".zh-cn." + selectedSub.format
		err := os.WriteFile(name, selectedSub.data, 0644)
		if err != nil {
			fmt.Printf("failed to write sub file: %v\n", err)
			continue
		}
		fmt.Printf("sub written to %v\n", name)
		if reference == "" && backupType != "" {
			reference = ffsubsync.FindBestReferenceSub(v)
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
		embyAPI.Refresh(v.Id, false)
	}
	time.Sleep(6 * time.Hour)
	goto start
}
