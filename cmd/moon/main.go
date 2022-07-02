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

	for i, v := range movieList {
		if i < 40 {
			continue
		}
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

		name := v.Path
		name = name[:len(name)-len(filepath.Ext(name))] + ".zh-cn" + filepath.Ext(subSorted[0].name)
		err := os.WriteFile(name, subSorted[0].data, 0644)
		if err != nil {
			print("failed to write sub file: ", err.Error(), "\n")
			continue
		}

		_, err = exec.LookPath("ffsubsync")
		if err == nil {
			var extSub string
			streams := make([]emby.EmbyVideoStream, len(v.MediaStreams))
			copy(streams, v.MediaStreams)
			for i := len(streams) - 1; i >= 0; i-- {
				ok := streams[i].Type == "Subtitle" && streams[i].IsExternal == false
				if ok == true {
					_, format := ffmpeg.SubtitleBestExtractFormat(streams[i].SubtitleCodecToFfmpeg())
					ok = format != ""
				}
				if ok == false {
					streams = append(streams[:i], streams[i+1:]...)
				}
			}
			if len(streams) > 0 {
				bestSub := streams[0]
				for i := len(streams) - 1; i >= 0; i-- {
					if streams[i].Codec == "PGSSUB" {
						streams = append(streams[:i], streams[i+1:]...)
					}
				}
				if len(streams) > 0 {
					bestSub = streams[0]
				}
				for i := len(streams) - 1; i >= 0; i-- {
					m, _ := regexp.MatchString("\bSDH\b", streams[i].Title)
					if m == true {
						streams = append(streams[:i], streams[i+1:]...)
					}
				}
				if len(streams) > 0 {
					bestSub = streams[0]
				}

				subData, err := ffmpeg.ExtractSubtitle(v.Path, bestSub.Index, bestSub.SubtitleCodecToFfmpeg())
				if err == nil {
					_, ext := ffmpeg.SubtitleBestExtractFormat(bestSub.SubtitleCodecToFfmpeg())
					if ext == "sup" {
						subData = pgstosrt.PgsToSrt(subData)
						ext = "srt"
					}
					name := strconv.Itoa(int(time.Now().Unix())) + "." + ext
					name = filepath.Join(os.TempDir(), name)
					err = os.WriteFile(name, subData, 0644)
					if err == nil {
						extSub = name
					}
				}
			}
			cmdArg := []string{v.Path, "-i", name, "--overwrite-input", "--reference-stream", "a:0", "--suppress-output-if-offset-less-than", "0.5"}
			if extSub != "" {
				cmdArg = []string{extSub, "-i", name, "--overwrite-input", "--suppress-output-if-offset-less-than", "0.5"}
			}
			cmd := exec.Command("ffsubsync", cmdArg...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			if extSub != "" {
				os.Remove(extSub)
			}
		}
		embyAPI.Refresh(v.Id, false)
	}
	time.Sleep(6 * time.Hour)
	goto start
}
