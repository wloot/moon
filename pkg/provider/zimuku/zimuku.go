package zimuku

import (
	"context"
	"encoding/base64"
	"fmt"
	"moon/pkg/cache"
	"moon/pkg/config"
	"moon/pkg/emby"
	"moon/pkg/episode"
	"moon/pkg/rod"
	"net/url"
	"sort"
	"strings"
	"time"

	rawRod "github.com/go-rod/rod"
	"github.com/otiai10/gosseract/v2"

	"regexp"
	"strconv"
)

const downloadNumbers = 2

type Zimuku struct {
	browser *rod.Rod
}

type subInfo struct {
	downloadElement *rawRod.Element
	downloadURL     string
	language        []string
	downloadCount   int
	votingScore     float64
	time            int64
	format          string
	name            string
}

func New() *Zimuku {
	z := &Zimuku{
		browser: rod.New(),
	}
	return z
}
func (z *Zimuku) Close() {
	z.browser.Close()
}

func (z *Zimuku) movieKeywords(movie emby.EmbyVideo) []string {
	var keywords []string
	if movie.ProviderIds.Imdb != "" {
		keywords = append(keywords, movie.ProviderIds.Imdb)
	} else {
		if movie.ProductionYear != 0 {
			keywords = append(keywords, movie.OriginalTitle+" ("+strconv.Itoa(movie.ProductionYear)+")")
			if movie.OriginalTitle != movie.Name {
				keywords = append(keywords, movie.Name+" ("+strconv.Itoa(movie.ProductionYear)+")")
			}
			// year offset +-1
			if movie.ProductionYear != time.Now().Year() {
				keywords = append(keywords, movie.OriginalTitle+" ("+strconv.Itoa(movie.ProductionYear+1)+")")
			}
			keywords = append(keywords, movie.OriginalTitle+" ("+strconv.Itoa(movie.ProductionYear-1)+")")
		}
	}
	return keywords
}

func (z *Zimuku) SeasonKeywords(season emby.EmbyVideo, series emby.EmbyVideo, episodes []emby.EmbyVideo) []string {
	var keywords []string

	if season.IndexNumber != 1 {
		for _, v := range episodes {
			if v.IndexNumber == 1 {
				if v.ProviderIds.Imdb != "" {
					keywords = append(keywords, v.ProviderIds.Imdb)
				}
				break
			}
		}
	} else {
		keywords = append(keywords, series.ProviderIds.Imdb)
	}

	originalTitle := series.OriginalTitle
	chineseTitle := series.Name
	seasonYear := strconv.Itoa(season.ProductionYear)
	seasonNumber := episode.ToChineseDigital(season.IndexNumber)

	if seasonYear != "0" && seasonYear != "" {
		keywords = append(keywords, "第"+seasonNumber+"季 "+originalTitle+" ("+seasonYear+")")
	}
	if originalTitle != chineseTitle {
		if seasonYear != "0" && seasonYear != "" {
			keywords = append(keywords, chineseTitle+" 第"+seasonNumber+"季 ("+seasonYear+")")
		}
	}
	if seasonNumber == "一" {
		keywords = append(keywords, originalTitle+" ("+seasonYear+")")
	}

	return keywords
}

func (z *Zimuku) SearchSeason(keywords []string, eps []emby.EmbyVideo) ([][]string, map[string]string) {
	var pageGC []*rawRod.Page
	defer func() {
		for i := range pageGC {
			pageGC[i].Close()
		}
	}()
	ctx, cancel := context.WithTimeout(z.browser.GetContext(), 10*time.Minute)
	defer cancel()

	var page *rawRod.Page
	err := rawRod.Try(func() {
		for _, k := range keywords {
			if k == "" {
				continue
			}
			fmt.Printf("zimuku: searching keyword %v\n", k)
			page = z.searchMainPage(ctx, &pageGC, k)
			if page == nil {
				continue
			}
			break
		}
	})
	if err != nil {
		fmt.Printf("zimuku: failed getting detail page, %v\n", err)
		return [][]string{}, map[string]string{}
	}
	if page == nil {
		fmt.Printf("zimuku: no detail page found, return\n")
		return make([][]string, len(eps)), map[string]string{}
	}

	var subs []subInfo
	err = rawRod.Try(func() {
		for childid := 1; true; childid++ {
			has, element, _ := page.Has("#subtb > tbody > tr:nth-child(" + strconv.Itoa(childid) + ")")
			if has == false {
				break
			}
			subs = append(subs, z.parseInfo(element))
		}
	})
	if err != nil {
		fmt.Printf("zimuku: parse detail page failed, %v\n", err)
		return [][]string{}, map[string]string{}
	}

	for i := len(subs) - 1; i >= 0; i-- {
		need := false
		for _, l := range subs[i].language {
			if l == "简体中文字幕" || l == "双语字幕" {
				need = true
				break
			}
		}
		if subs[i].format == "sup" {
			need = false
		}
		if need == false {
			subs = append(subs[:i], subs[i+1:]...)
		}
	}
	if len(subs) == 0 {
		fmt.Printf("zimuku: no sub for now\n")
		return make([][]string, len(eps)), map[string]string{}
	}

	var perEp = make(map[int][]subInfo)
	for _, v := range subs {
		ep := episode.NameToEpisode(v.name)
		if ep <= 0 {
			ep = 0
		}
		perEp[ep] = append(perEp[ep], v)
	}

	var out [][]string
	info := make(map[string]string)
	for i := range eps {
		var subs []subInfo
		e := eps[i].IndexNumber
		if _, ok := perEp[e]; ok {
			subs = append(subs, perEp[e]...)
		}
		if _, ok := perEp[0]; ok {
			subs = append(subs, perEp[0]...)
		}
		if len(subs) == 0 {
			out = append(out, []string{})
			continue
		}
		sort.Slice(subs, func(i, j int) bool {
			less := subs[i].downloadCount > subs[j].downloadCount
			if less == true && subs[i].votingScore <= 5 && subs[i].votingScore > 0 {
				if subs[j].votingScore > subs[i].votingScore {
					less = false
				}
			}
			return less
		})
		downloadNumbers := downloadNumbers - 1
		if _, ok := perEp[0]; ok {
			downloadNumbers += 1
		} else if downloadNumbers < 1 {
			downloadNumbers = 1
		}
		var subFiles []string
		for i, v := range subs {
			if i >= downloadNumbers || i > 5 {
				break
			}
			if deadline, ok := page.GetContext().Deadline(); ok && time.Until(deadline) <= 0 {
				fmt.Printf("zimuku: stop download as main context timeout\n")
				if len(subFiles) > 0 {
					out = append(out, subFiles)
				}
				return out, info
			}
			file := cache.TryGet(cache.MergeKeys("zimuku", v.downloadURL), "downloads", func() string {
				fmt.Printf("zimuku: downlaoding sub, %v\n", v)
				retry := false
			seasondown:
				var file string
				ctx, cancel := context.WithTimeout(z.browser.GetContext(), 30*time.Second)
				err := rawRod.Try(func() {
					file = z.downloadSub(ctx, &pageGC, page, v.downloadElement)
				})
				cancel()

				if file == "" {
					if retry == false {
						retry = true
						goto seasondown
					} else {
						downloadNumbers += 1
						if err != nil {
							fmt.Printf("zimuku: sub download failed, %v\n", err)
						} else {
							fmt.Printf("zimuku: sub download failed, no file\n")
						}
					}
				}
				return file
			})
			if file != "" {
				subFiles = append(subFiles, file)
				if _, ok := info[file]; !ok {
					info[file] = v.name
				}
			}
		}
		if len(subFiles) > 0 {
			out = append(out, subFiles)
		} else {
			return out, info
		}
	}
	return out, info
}

func (z *Zimuku) SearchMovie(movie emby.EmbyVideo) ([]string, bool) {
	var pageGC []*rawRod.Page
	defer func() {
		for i := range pageGC {
			pageGC[i].Close()
		}
	}()
	ctx, cancel := context.WithTimeout(z.browser.GetContext(), 3*time.Minute)
	defer cancel()

	var page *rawRod.Page
	keywords := z.movieKeywords(movie)
	if len(keywords) == 0 {
		return []string{}, false
	}
	err := rawRod.Try(func() {
		for _, k := range keywords {
			if k == "" {
				continue
			}
			fmt.Printf("zimuku: searching keyword %v\n", k)
			page = z.searchMainPage(ctx, &pageGC, k)
			if page == nil {
				continue
			}
			break
		}
	})
	if err != nil {
		fmt.Printf("zimuku: failed getting detail page, %v\n", err)
		return []string{}, true
	}
	if page == nil {
		fmt.Printf("zimuku: no detail page found, return\n")
		return []string{}, false
	}

	var subs []subInfo
	err = rawRod.Try(func() {
		for childid := 1; true; childid++ {
			has, element, _ := page.Has("#subtb > tbody > tr:nth-child(" + strconv.Itoa(childid) + ")")
			if has == false {
				break
			}
			subs = append(subs, z.parseInfo(element))
		}
	})
	if err != nil {
		fmt.Printf("zimuku: parse detail page failed, %v\n", err)
		return []string{}, true
	}

	for i := len(subs) - 1; i >= 0; i-- {
		need := false
		for _, l := range subs[i].language {
			if l == "简体中文" || l == "双语" {
				need = true
				break
			}
		}
		if subs[i].format == "sup" {
			need = false
		}
		if need == false {
			subs = append(subs[:i], subs[i+1:]...)
		}
	}
	if len(subs) == 0 {
		fmt.Printf("zimuku: no sub for now\n")
		return []string{}, false
	}

	firstTime := subs[len(subs)-1].time
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].time-subs[j].time > 0 {
			if subs[j].time-firstTime < 604800 { // 7 days
				return true
			}
		}
		less := subs[i].downloadCount > subs[j].downloadCount
		if less == true && subs[i].votingScore <= 5 && subs[i].votingScore > 0 {
			if subs[j].votingScore > subs[i].votingScore {
				less = false
			}
		}
		return less
	})

	if config.DEBUG == true {
		fmt.Printf("zimuku: all sub grabed are %v\n", subs)
	}

	var subFiles []string
	downloadNumbers := downloadNumbers
	for i, v := range subs {
		if i >= downloadNumbers || i > 5 {
			break
		}
		if deadline, ok := page.GetContext().Deadline(); ok && time.Until(deadline) <= 0 {
			fmt.Printf("zimuku: stop download as main context timeout\n")
			if len(subFiles) == 0 {
				return subFiles, true
			} else {
				return subFiles, false
			}
		}
		file := cache.TryGet(cache.MergeKeys("zimuku", v.downloadURL), "downloads", func() string {
			fmt.Printf("zimuku: downlaoding sub, %v\n", v)
			retry := false
		moviedown:
			var file string
			ctx, cancel := context.WithTimeout(z.browser.GetContext(), 30*time.Second)
			err := rawRod.Try(func() {
				file = z.downloadSub(ctx, &pageGC, page, v.downloadElement)
			})
			cancel()

			if file == "" {
				if retry == false {
					retry = true
					goto moviedown
				} else {
					downloadNumbers += 1
					if err != nil {
						fmt.Printf("zimuku: sub download failed, %v\n", err)
					} else {
						fmt.Printf("zimuku: sub download failed, no file\n")
					}
				}
			}
			return file
		})
		if file != "" {
			subFiles = append(subFiles, file)
		}
	}
	if len(subFiles) == 0 {
		return subFiles, true
	} else {
		return subFiles, false
	}
}

func (z *Zimuku) downloadSub(ctx context.Context, gc *[]*rawRod.Page, prePage *rawRod.Page, preElement *rawRod.Element) string {
	wait := prePage.Context(ctx).MustWaitOpen()
	preElement.Context(ctx).MustClick()
	page := wait()
	*gc = append(*gc, page)

	element := page.MustElement("#down1")
	element.MustEval(`() => { this.target = "" }`)
	element.MustScrollIntoView()
	page.Mouse.Scroll(0, 50/2, 1)
	element.MustClick()
	file := z.browser.HookDownload(func() {
		page.MustElement("body > main > div > div > div > table > tbody > tr > td:nth-child(1) > div > ul > li:nth-child(2) > a").MustClick()
	})
	page.Close()
	return file
}

func (z *Zimuku) parseInfo(element *rawRod.Element) subInfo {
	sub := subInfo{}
	sub.downloadElement = element.MustElement("td.first > a")
	sub.downloadURL = *sub.downloadElement.MustAttribute("href")
	sub.name = *sub.downloadElement.MustAttribute("title")

	format := element.MustElement("td.first > span:nth-child(2)").MustText()
	if has, _, _ := element.Has("td.first > span:nth-child(3)"); has == false {
		if format == "ASS/SSA" {
			sub.format = "ass"
		}
		if format == "SRT" {
			sub.format = "srt"
		}
		if format == "SUP" {
			sub.format = "sup"
		}
	}

	for langid := 1; true; langid++ {
		has, image, _ := element.Has("td.tac.lang > img:nth-child(" + strconv.Itoa(langid) + ")")
		if has == false {
			break
		}
		sub.language = append(sub.language, *image.MustAttribute("alt"))
	}

	voting := *element.MustElements("td.tac.hidden-xs")[0].MustElement("i").MustAttribute("data-original-title")
	sub.votingScore, _ = strconv.ParseFloat(regexp.MustCompile("[0-9.]+").FindString(voting), 64)

	count := element.MustElements("td.tac.hidden-xs")[1].MustText()
	if strings.HasSuffix(count, "万") {
		count = count[:len(count)-len("万")]
		countf, _ := strconv.ParseFloat(count, 64)
		countf = countf * 10000
		sub.downloadCount = int(countf)
	} else {
		sub.downloadCount, _ = strconv.Atoi(count)
	}

	date := element.MustElement("td.last.hidden-xs").MustText()
	date = regexp.MustCompile(" .*\n (.+)").FindStringSubmatch(date)[1]
	if strings.HasSuffix(date, "天前") {
		date = date[:len(date)-len("天前")]
		datei, _ := strconv.ParseInt(date, 10, 64)
		sub.time = time.Now().Add(-time.Duration(datei) * time.Hour * 24).Unix()
	}
	if strings.HasSuffix(date, "小时前") {
		date = date[:len(date)-len("小时前")]
		datei, _ := strconv.ParseInt(date, 10, 64)
		sub.time = time.Now().Add(-time.Duration(datei) * time.Hour).Unix()
	}
	if strings.HasSuffix(date, "分钟前") {
		date = date[:len(date)-len("分钟前")]
		datei, _ := strconv.ParseInt(date, 10, 64)
		sub.time = time.Now().Add(-time.Duration(datei) * time.Minute).Unix()
	}
	if date == "刚刚" {
		sub.time = time.Now().Unix()
	}
	if t, err := time.Parse("06/1/2", date); err == nil {
		sub.time = t.Unix() + 28800 // UTC + 8
	}
	if t, err := time.Parse("1月2日2006", date+strconv.Itoa(time.Now().Year())); err == nil {
		sub.time = t.Unix() + 28800 // UTC + 8
	}

	return sub
}

func (z *Zimuku) searchMainPage(ctx context.Context, gc *[]*rawRod.Page, keyword string) *rawRod.Page {
	page := z.browser.Context(ctx).MustPage("https://srtku.com/search?q=" + url.QueryEscape(keyword))
	*gc = append(*gc, page)
	resolveCaptcha(page)

	page.MustWaitLoad()
	has, element, _ := page.Has("body > div.container > div > div > div.box.clearfix > div:nth-child(2) > div.title > p.tt.clearfix > a")
	if has == false {
		page.Close()
		return nil
	}
	element.MustEval(`() => { this.target = "" }`)
	element.MustClick()
	page.MustWaitLoad()

	return page
}

func resolveCaptcha(page *rawRod.Page) {
	resolveTimes := 0
	for resolveTimes < 5 {
		page.WaitElementsMoreThan("div", 1)
		page.MustWaitLoad()
		has, element, _ := page.Has("body > div > div:nth-child(4) > table > tbody > tr:nth-child(1) > td:nth-child(3) > img")
		if has == true {
			img := *element.MustAttribute("src")
			img = img[len("data:image/bmp;base64,"):]
			b, err := base64.StdEncoding.DecodeString(img)
			var text string
			if err == nil {
				// CGO: start
				client := gosseract.NewClient()
				client.SetImageFromBytes(b)
				//client.SetWhitelist("0123456789")
				text, err = client.Text()
				client.Close()
				// CGO: end
			}
			if err != nil {
				fmt.Printf("zimuku: verify error: %v\n", err)
				page.MustReload()
			} else {
				fmt.Printf("zimuku: verify code: resolved '%v'\n", text)
				page.MustElement("#intext").MustInput(text)
				page.MustElement("body > div > div:nth-child(4) > table > tbody > tr:nth-child(2) > td > input[type=submit]").MustClick()
			}
			resolveTimes += 1
		} else {
			break
		}
	}
}
