package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"moon/pkg/api/emby"
	"moon/pkg/charset"
	"moon/pkg/ffmpeg"
	"moon/pkg/pgstosrt"
	"moon/pkg/provider/zimuku"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
	"github.com/go-rod/rod/lib/utils"
	"github.com/mholt/archiver/v4"
)

type Subinfo struct {
	name    string
	data    []byte
	info    *astisub.Subtitles
	chinese bool
	double  bool
	tc      bool
}

var SETTNGS_videopath_map map[string]string = map[string]string{}

var SETTINGS_emby_url string = "http://play.charontv.com"
var SETTINGS_emby_key string = "fe1a0f6c143043e98a1f3099bfe0a3a8"
var SETTINGS_emby_importcount int = 10

func main() {
	emby := emby.New(SETTINGS_emby_url, SETTINGS_emby_key)
	zimuku := zimuku.New()

	for ii, id := range emby.RecentMovie(SETTINGS_emby_importcount) {
		if ii == 0 {
			continue
		}
		emby.Refresh(id, true)
		time.Sleep(10 * time.Second)
		v := emby.MovieInfo(id)
		for old, new := range SETTNGS_videopath_map {
			if strings.HasPrefix(v.Path, old) {
				v.Path = new + v.Path[len(old):]
			}
		}
		subFiles := zimuku.SearchMovie(v)
		var subSorted []Subinfo
		for _, f := range subFiles {
			fsys, err := archiver.FileSystem(f)
			if err != nil {
				continue
			}
			fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if d.IsDir() == true {
					return nil
				}

				data, _ := fs.ReadFile(fsys, path)
				if transformed, err := charset.AnyToUTF8(data); err == nil {
					data = transformed
				}

				var s *astisub.Subtitles
				switch filepath.Ext(strings.ToLower(d.Name())) {
				case ".srt":
					s, err = astisub.ReadFromSRT(bytes.NewReader(data))
				case ".ssa", ".ass":
					s, err = astisub.ReadFromSSA(bytes.NewReader(data))
				default:
					return nil
				}
				if err != nil {
					return nil
				}

				subSorted = append(subSorted, Subinfo{
					data: data,
					info: s,
					name: d.Name(),
				})
				return nil
			})
		}
		if len(subSorted) == 0 {
			continue
		}

		jianfan := charset.NewJianfan()
		for i := range subSorted {
			countTC := 0
			countChars := 0
			countCh := 0
			countLines := 0
			durationLast := make(map[string]struct{})
			for _, v := range subSorted[i].info.Items {
				if len(v.Lines) == 0 {
					continue
				}

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

			subSorted[i].chinese = true
			if countLines/2 > countCh {
				subSorted[i].chinese = false
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
			return false
		})

		name := v.Path
		name = name[:len(name)-len(filepath.Ext(name))] + ".zh-cn" + filepath.Ext(subSorted[0].name)
		err := os.WriteFile(name, subSorted[0].data, 0644)
		if err != nil {
			print("failed to write sub file: ", err.Error(), "\n")
			continue
		}

		emby.Refresh(id, false)
		_, err = exec.LookPath("ffsubsync")
		if err == nil || true {
			var extSub string
			print("we are ffmpeg.ProbeVideo() ", v.Path, "\n")
			streams, err := ffmpeg.ProbeVideo(v.Path)
			if err != nil {
				print("ffmpeg.ProbeVideo() err:", err.Error(), "\n")
			}
			for i := range streams {
				fmt.Printf("0 stream%d: %v\n", i, streams[i])
			}
			for i := len(streams) - 1; i >= 0; i-- {
				ok := streams[i].CodecType == "subtitle"
				if ok == true {
					_, ok = ffmpeg.SubtitleCodecToFormat[streams[i].CodecName]
				}
				if ok == false {
					streams = append(streams[:i], streams[i+1:]...)
				}
			}
			for i := range streams {
				fmt.Printf("1 stream%d: %v\n", i, streams[i])
			}
			if len(streams) > 0 {
				bestSub := streams[0]
				for i := len(streams) - 1; i >= 0; i-- {
					if streams[i].CodecName == "hdmv_pgs_subtitle" {
						streams = append(streams[:i], streams[i+1:]...)
					}
				}
				for i := range streams {
					fmt.Printf("2 stream%d: %v\n", i, streams[i])
				}
				if len(streams) > 0 {
					bestSub = streams[0]
				}
				for i := len(streams) - 1; i >= 0; i-- {
					m, _ := regexp.MatchString("\bSDH\b", streams[i].Tags.Title)
					if m == true {
						streams = append(streams[:i], streams[i+1:]...)
					}
				}
				for i := range streams {
					fmt.Printf("3 stream%d: %v\n", i, streams[i])
				}
				if len(streams) > 0 {
					bestSub = streams[0]
				}
				fmt.Printf("bestSub: %v\n", bestSub)
				subData, err := ffmpeg.ExtractSubtitle(v.Path, bestSub)
				if err == nil {
					fmt.Printf("subdata %v\n", string(subData))
					if bestSub.CodecName == "hdmv_pgs_subtitle" {
						subData = pgstosrt.PgsToSrt(subData)
						fmt.Printf("subdata2 %v\n", string(subData))
					}
					name := strconv.Itoa(int(time.Now().Unix())) + "." + ffmpeg.SubtitleCodecToFormat[bestSub.CodecName]
					name = filepath.Join(os.TempDir(), name)
					err = os.WriteFile(name, subData, 0644)
					if err == nil {
						extSub = name
					} else {
						fmt.Printf("os.WriteFile err: %v\n", err)
					}
				} else {
					fmt.Printf("ffmpeg.ExtractSubtitle err: %v\n", err)
				}
			}
			cmdArg := []string{v.Path, "-i", name, "--overwrite-input", "--vad", "webrtc"}
			if extSub != "" {
				cmdArg = []string{extSub, "-i", name, "--overwrite-input"}
			}
			cmd := exec.Command("ffsubsync", cmdArg...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			print(extSub)
			//if extSub != "" {
			//	os.Remove(extSub)
			//}
		}
	}
	utils.Pause()
}
