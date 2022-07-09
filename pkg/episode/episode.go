package episode

import (
	"regexp"
	"strconv"
	"strings"
)

func NameToSeason(name string) int {
	return -1
}

func NameToEpisode(name string) int {
	if regexp.MustCompile(`(\b|\d)[eE]\d+-[eE]\d+\b`).MatchString(name) {
		return -1
	}
	if regexp.MustCompile(`全([一二三四五六七八九十]+|[\d]+)集`).MatchString(name) {
		return -1
	}
	match := regexp.MustCompile(`(\b|\d)[eE](\d+)\b`).FindStringSubmatch(name)
	if len(match) == 3 {
		i, err := strconv.ParseInt(match[2], 10, 64)
		if err == nil {
			return int(i)
		}
	}
	match = regexp.MustCompile(`第(\d+)集`).FindStringSubmatch(name)
	if len(match) == 2 {
		i, err := strconv.ParseInt(match[1], 10, 64)
		if err == nil {
			return int(i)
		}
	}
	match = regexp.MustCompile(`第(.+)集`).FindStringSubmatch(name)
	if len(match) == 2 {
		i, err := strconv.ParseInt(fromChineseDigital(match[1]), 10, 64)
		if err == nil {
			return int(i)
		}
	}
	return -1
}

// 一 - 九十九
func fromChineseDigital(c string) string {
	var chineseDigital = map[rune]rune{
		'一': '1',
		'二': '2',
		'三': '3',
		'四': '4',
		'五': '5',
		'六': '6',
		'七': '7',
		'八': '8',
		'九': '9',
	}

	c = strings.Map(func(r rune) rune {
		if d, ok := chineseDigital[r]; ok {
			return d
		}
		return r
	}, c)

	if strings.HasPrefix(c, "十") {
		c = strings.Replace(c, "十", "1", 1)
	} else if strings.HasSuffix(c, "十") {
		c = strings.Replace(c, "十", "0", 1)
	} else {
		c = strings.Replace(c, "十", "", 1)
	}

	return c
}

// 1 - 99
func ToChineseDigital(i int) string {
	var chineseDigital = map[rune]rune{
		'1': '一',
		'2': '二',
		'3': '三',
		'4': '四',
		'5': '五',
		'6': '六',
		'7': '七',
		'8': '八',
		'9': '九',
	}

	c := strconv.FormatInt(int64(i), 10)
	if i >= 10 {
		c10 := strconv.FormatInt(int64(i/10), 10)
		c1 := strconv.FormatInt(int64(i%10), 10)
		c = c10 + "十" + c1
	}

	c = strings.Map(func(r rune) rune {
		if d, ok := chineseDigital[r]; ok {
			return d
		}
		return r
	}, c)

	if i >= 10 {
		if strings.HasPrefix(c, "一") {
			c = strings.Replace(c, "一", "", 1)
		}
		if strings.HasSuffix(c, "0") {
			c = strings.Replace(c, "0", "", 1)
		}
	}

	return c
}
