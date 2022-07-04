package zimuku

import (
	"context"
	"fmt"
	"moon/pkg/config"
	"moon/pkg/emby"
	"moon/pkg/rod"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	rawRod "github.com/go-rod/rod"

	"regexp"
	"strconv"
)

var downloadNumbers = 3

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
}

func New() Zimuku {
	return Zimuku{
		browser: rod.New(),
	}
}

func (z *Zimuku) SearchMovie(movie emby.EmbyVideo) []string {
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
			keywords = append(keywords, movie.OriginalTitle+" ("+strconv.Itoa(movie.ProductionYear+1)+")")
			keywords = append(keywords, movie.OriginalTitle+" ("+strconv.Itoa(movie.ProductionYear-1)+")")
		}
	}

	var pageGC []*rawRod.Page
	defer func() {
		for i := range pageGC {
			pageGC[i].Close()
		}
	}()
	ctx, cancel := context.WithTimeout(z.browser.GetContext(), 5*time.Minute)
	defer cancel()

	var page *rawRod.Page
	err := rawRod.Try(func() {
		for _, k := range keywords {
			fmt.Printf("zimuku: searching keyword %v\n", k)
			page = z.searchMainPage(ctx, k, pageGC)
			if page == nil {
				fmt.Printf("zimuku: searching faild, not found\n")
				continue
			}
			break
		}
		page.MustWaitLoad()
	})
	if err != nil {
		fmt.Printf("zimuku: failed getting detail page, %v\n", err)
		return []string{}
	}
	if page == nil {
		fmt.Printf("zimuku: no detail page found, return\n")
		return []string{}
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
		return []string{}
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
		return []string{}
	}

	firstTime := subs[len(subs)-1].time
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].time-subs[j].time > 0 {
			if subs[j].time-firstTime < 604800 { // 7 days
				return true
			}
		}
		less := subs[i].downloadCount >= subs[j].downloadCount
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
	for i, v := range subs {
		if i >= downloadNumbers {
			break
		}
		ctx, cancel := context.WithTimeout(page.GetContext(), 30*time.Second)
		fmt.Printf("zimuku: downlaoding sub, %v\n", v)
		err := rawRod.Try(func() {
			wait := page.Context(ctx).MustWaitOpen()
			v.downloadElement.MustEval(`() => { this.target = "_blank" }`)
			v.downloadElement.MustClick()
			page := wait()
			pageGC = append(pageGC, page)

			element := page.MustElement("#down1")
			element.MustEval(`() => { this.target = "" }`)
			element.MustScrollIntoView()
			page.Mouse.Scroll(0, 50/2, 1)
			element.MustClick()
			file := z.browser.HookDownload(func() {
				page.MustElement("body > main > div > div > div > table > tbody > tr > td:nth-child(1) > div > ul > li:nth-child(1) > a").MustClick()
			})
			if file != "" {
				if ext := filepath.Ext(file); ext == "" && v.format != "" {
					fmt.Printf("zimuku: sub has no ext, use %v\n", v.format)
					os.Rename(file, file+"."+v.format)
					file = file + "." + v.format
				}
				subFiles = append(subFiles, file)
			} else {
				downloadNumbers += 1
				fmt.Printf("zimuku: sub download failed, no file\n")
			}
		})
		cancel()
		if err != nil {
			downloadNumbers += 1
			fmt.Printf("zimuku: sub download failed, %v\n", err)
		}
	}
	return subFiles
}

func (z *Zimuku) parseInfo(element *rawRod.Element) subInfo {
	sub := subInfo{}
	sub.downloadElement = element.MustElement("td.first > a")
	sub.downloadURL = *element.MustElement("td.first > a").MustAttribute("href")
	count := element.MustElement("td:nth-child(4)").MustText()
	if strings.HasSuffix(count, "万") {
		count = count[:len(count)-len("万")]
		countf, _ := strconv.ParseFloat(count, 64)
		countf = countf * 10000
		sub.downloadCount = int(countf)
	} else {
		sub.downloadCount, _ = strconv.Atoi(count)
	}
	date := element.MustElement("td:nth-child(5)").MustText()
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
	votingStr := *element.MustElement("td:nth-child(3) > i").MustAttribute("data-original-title")
	sub.votingScore, _ = strconv.ParseFloat(regexp.MustCompile("[0-9.]+").FindString(votingStr), 64)
	return sub
}

func (z *Zimuku) searchMainPage(ctx context.Context, keyword string, gc []*rawRod.Page) *rawRod.Page {
	page := z.browser.Context(ctx).MustPage("https://zimuku.org/")
	gc = append(gc, page)
	// 搜索框输入
	page.MustElement("body > div.navbar.navbar-inverse.navbar-static-top > div > div.navbar-header > div > form > div > input").MustInput(keyword)
	// 搜索按钮
	page.MustElement("body > div.navbar.navbar-inverse.navbar-static-top > div > div.navbar-header > div > form > div > span > button").MustClick()

	page.WaitElementsMoreThan("button", 1) // if first access
	// 搜索结果页第一个结果
	has, element, _ := page.Has("body > div.container > div > div > div.box.clearfix > div:nth-child(2) > div.litpic.hidden-xs > a")
	if has == false {
		return nil
	}
	element.MustEval(`() => { this.target = "" }`)
	element.MustClick()

	return page
}
