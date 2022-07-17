package subtitle

import (
	"moon/pkg/charset"
	"regexp"
	"strings"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
)

func AnalyzeASS(info *astisub.Subtitles) SubContent {
	jianfan := charset.NewJianfan()
	perStyle := make(map[string]*struct {
		countItems     int // 所有独立的对白数, 用于计算是否是中文字幕
		countAllLines  int // 所有的行数, 用于计算是否是双语字幕
		countChiFirst  int // 第一行为中文的对白数
		countChiSecond int // 第二行为中文的对白数
		countChiChars  int // 中文行的字数
		countChtChars  int // 中文行中繁体的字数
	})
	for _, item := range info.Items {
		var lines []string
		l := item.Lines[0].String()
		l = regexp.MustCompile(`(?m)({[^}]*})`).ReplaceAllString(l, "")
		l = regexp.MustCompile(`(?m)(\<[^>]*\>)`).ReplaceAllString(l, "")
		l = strings.ReplaceAll(l, `\n`, `\N`)
		for _, line := range strings.Split(l, `\N`) {
			if line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			continue
		}

		style := item.Style.ID
		if perStyle[style] == nil {
			perStyle[style] = &struct {
				countItems     int
				countAllLines  int
				countChiFirst  int
				countChiSecond int
				countChiChars  int
				countChtChars  int
			}{}
		}
		perStyle[style].countItems += 1
		perStyle[style].countAllLines += len(lines)
		for i, line := range lines {
			if i > 1 {
				break
			}
			lang := whatlanggo.Detect(line)
			if lang.Lang == whatlanggo.Cmn {
				if i == 0 {
					perStyle[style].countChiFirst += 1
				}
				if i == 1 {
					perStyle[style].countChiSecond += 1
				}
				perStyle[style].countChiChars += len([]rune(line))
				perStyle[style].countChtChars += jianfan.CountCht(line)
				break
			}
		}
	}

	var analyze SubContent
	mostItems := 0
	for _, p := range perStyle {
		if mostItems < p.countItems {
			mostItems = p.countItems
		}
	}
	for _, p := range perStyle {
		if p.countItems*3 < mostItems*2 {
			continue
		}
		if p.countItems/2 < p.countChiFirst {
			analyze.Chinese = true
		}
		if p.countItems/2 < p.countChiSecond {
			analyze.Chinese = true
			analyze.OriFirst = true
		}
		if p.countChiChars/10 < p.countChtChars {
			analyze.Cht = true
		}
		if p.countItems*3 < p.countAllLines*2 {
			analyze.Double = true
		}
	}
	if analyze.Double == false {
		for _, p := range perStyle {
			if p.countItems*3 < mostItems*2 {
				continue
			}
			if !(p.countItems/2 < p.countChiFirst+p.countChiSecond) {
				analyze.Double = true
			}
		}
	}
	if analyze.Double == false && analyze.OriFirst == true {
		analyze.Chinese = false
		analyze.OriFirst = false
	}
	if analyze.Cht == true && analyze.Chinese == false {
		analyze.Cht = false
	}
	return analyze
}
