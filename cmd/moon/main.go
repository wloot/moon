package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"moon/pkg/api/emby"
	"moon/pkg/charset"
	"moon/pkg/provider/zimuku"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

var SETTNGS_videopath_map map[string]string = map[string]string{
	"/gd/": "/Users/wloot/gd/",
}

var SETTINGS_emby_url string = "http://play.charontv.com"
var SETTINGS_emby_key string = "fe1a0f6c143043e98a1f3099bfe0a3a8"
var SETTINGS_emby_importcount int = 10

func main() {
	emby := emby.New(SETTINGS_emby_url, SETTINGS_emby_key)
	zimuku := zimuku.New()

	for _, v := range emby.MovieItems(SETTINGS_emby_importcount) {
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
					//data: data,
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

		fmt.Printf("%+v\n", subSorted)

		name := v.Path
		name = name[0:len(name)-len(filepath.Ext(name))] + ".zh-cn" + filepath.Ext(subSorted[0].name)
		err := ioutil.WriteFile(name, subSorted[0].data, 0644)
		if err != nil {
			print(err, "\n")
		}

		time.Sleep(5)
		_, err = exec.LookPath("ffsubsync")
		if err == nil {
			cmd := exec.Command("ffsubsync '" + v.Path + "' -i '" + name + "' --overwrite-input")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			log.Println(cmd.Run())
		}
	}
	utils.Pause()
}
