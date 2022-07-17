package subtitle

import (
	"moon/pkg/charset"
	"regexp"
	"strings"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
)

func AnalyzeSRT(info *astisub.Subtitles) SubContent {
	jianfan := charset.NewJianfan()
	countItems := 0     // 所有独立的对白数, 用于计算是否是中文字幕
	countChiFirst := 0  // 第一行为中文的对白数
	countChiSecond := 0 // 第二行为中文的对白数
	countAllLines := 0  // 所有的行数, 用于计算是否是双语字幕
	countChiChars := 0  // 中文行的字数
	countChtChars := 0  // 中文行中繁体的字数
	for _, item := range info.Items {
		var lines []string
		for i := range item.Lines {
			l := item.Lines[i].String()
			l = regexp.MustCompile(`(?m)({[^}]*})`).ReplaceAllString(l, "")
			l = regexp.MustCompile(`(?m)(\<[^>]*\>)`).ReplaceAllString(l, "")
			l = strings.ReplaceAll(l, `\n`, `\N`)
			for _, line := range strings.Split(l, `\N`) {
				if line != "" {
					lines = append(lines, line)
				}
			}
		}
		if len(lines) == 0 {
			continue
		}
		countAllLines += len(lines)

		countItems += 1
		for i, line := range lines {
			if i > 1 {
				break
			}
			lang := whatlanggo.Detect(line)
			if lang.Lang == whatlanggo.Cmn {
				if i == 0 {
					countChiFirst += 1
				}
				if i == 1 {
					countChiSecond += 1
				}
				countChiChars += len([]rune(line))
				countChtChars += jianfan.CountCht(line)
				break
			}
		}
	}

	var analyze SubContent
	if countItems/2 < countChiFirst {
		analyze.Chinese = true
	}
	if countItems/2 < countChiSecond {
		analyze.Chinese = true
		analyze.OriFirst = true
	}
	if countChiChars/10 < countChtChars {
		analyze.Cht = true
	}
	if countItems*3 < countAllLines*2 {
		analyze.Double = true
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
