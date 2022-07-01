package zimuku

import (
	"errors"
	"fmt"
	"moon/pkg/client/rod"
	"moon/pkg/video"
	"sort"
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
}

func New() Zimuku {
	return Zimuku{
		browser: rod.New(),
	}
}

func (z *Zimuku) SearchMovie(movie video.Movie) []string {
	var keywords []string
	if movie.ImdbId != "" {
		keywords = append(keywords, movie.ImdbId)
	}
	if movie.Year != 0 {
		for _, t := range movie.Titles {
			keywords = append(keywords, t+" ("+strconv.Itoa(movie.Year)+")")
		}
		// year offset +-1
		keywords = append(keywords, movie.Titles[0]+" ("+strconv.Itoa(movie.Year+1)+")")
		keywords = append(keywords, movie.Titles[0]+" ("+strconv.Itoa(movie.Year-1)+")")
	}

	var page *rawRod.Page
	for _, k := range keywords {
		fmt.Printf("zimuku: searching keyword %v\n", k)
		var err error
		page, err = z.searchMainPage(k)
		if err != nil {
			fmt.Printf("zimuku: searching faild, %v\n", err)
			continue
		}
		break
	}
	if page == nil {
		fmt.Printf("zimuku: no detail page found, return\n")
		return []string{}
	}

	for failed := false; true; {
		err := page.Timeout(5 * time.Second).WaitLoad()
		if err != nil {
			if failed == true {
				page.MustClose()
				fmt.Printf("zimuku: detail page load failed, return\n")
				return []string{}
			}
			page.Reload()
			failed = true
			continue
		}
		break
	}
	var subs []subInfo
	for childid := 1; true; childid++ {
		has, element, _ := page.Has("#subtb > tbody > tr:nth-child(" + strconv.Itoa(childid) + ")")
		if has == false {
			break
		}
		sub := subInfo{}
		sub.downloadElement = element.MustElement("td.first > a")
		sub.downloadURL = *element.MustElement("td.first > a").MustAttribute("href")
		sub.downloadCount, _ = strconv.Atoi(element.MustElement("td:nth-child(4)").MustText())
		for langid := 1; true; langid++ {
			has, image, _ := element.Has("td.tac.lang > img:nth-child(" + strconv.Itoa(langid) + ")")
			if has == false {
				break
			}
			sub.language = append(sub.language, *image.MustAttribute("alt"))
		}
		votingStr := *element.MustElement("td:nth-child(3) > i").MustAttribute("data-original-title")
		sub.votingScore, _ = strconv.ParseFloat(regexp.MustCompile("[0-9.]+").FindString(votingStr), 64)
		subs = append(subs, sub)
	}

	for i := len(subs) - 1; i >= 0; i-- {
		need := false
		for _, l := range subs[i].language {
			if l == "简体中文" || l == "双语" {
				need = true
				break
			}
		}
		if need == false {
			subs = append(subs[:i], subs[i+1:]...)
		}
	}

	sort.Slice(subs, func(i, j int) bool {
		less := subs[i].downloadCount >= subs[j].downloadCount
		if less == true && subs[i].votingScore <= 5 && subs[i].votingScore > 0 {
			if subs[j].votingScore > subs[i].votingScore {
				less = false
			}
		}
		return less
	})

	var subFiles []string
	for i, v := range subs {
		if i >= downloadNumbers {
			break
		}
		fmt.Printf("zimuku: downlaoding sub, %v\n", v)
		err := rawRod.Try(func() {
			wait := page.MustWaitOpen()
			v.downloadElement.MustEval(`() => { this.target = "_blank" }`)
			v.downloadElement.MustClick()
			page := wait()
			page.Timeout(5 * time.Second).MustWaitLoad()

			element := page.MustElement("#down1")
			element.MustEval(`() => { this.target = "" }`)
			element.MustScrollIntoView()
			page.Mouse.Scroll(0, 50/2, 1)
			element.MustClick()
			file := z.browser.HookDownload(func() {
				page.MustElement("body > main > div > div > div > table > tbody > tr > td:nth-child(1) > div > ul > li:nth-child(5) > a").MustClick()
			})
			page.MustClose()
			if file != "" {
				subFiles = append(subFiles, file)
			}
		})
		if err != nil {
			downloadNumbers += 1
			fmt.Printf("zimuku: sub download failed, %v\n", err)
		}
	}
	page.MustClose()
	return subFiles
}

func (z *Zimuku) searchMainPage(keyword string) (*rawRod.Page, error) {
	page := z.browser.MustPage("https://zimuku.org/")
	err := page.Timeout(5*time.Second).WaitElementsMoreThan("button", 0) // if first access
	if err != nil {
		page.MustClose()
		return nil, err
	}
	// 搜索框输入
	page.MustElement("body > div.navbar.navbar-inverse.navbar-static-top > div > div.navbar-header > div > form > div > input").MustInput(keyword)
	// 搜索按钮
	page.MustElement("body > div.navbar.navbar-inverse.navbar-static-top > div > div.navbar-header > div > form > div > span > button").MustClick()

	err = page.Timeout(5 * time.Second).WaitLoad()
	if err != nil {
		page.MustClose()
		return nil, err
	}
	// 搜索结果页第一个结果
	has, element, _ := page.Has("body > div.container > div > div > div.box.clearfix > div:nth-child(2) > div.litpic.hidden-xs > a")
	if has == false {
		page.MustClose()
		return nil, errors.New("not found")
	}
	element.MustEval(`() => { this.target = "" }`)
	element.MustClick()

	return page, nil
}
