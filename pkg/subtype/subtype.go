package subtype

import (
	"regexp"
	"strings"
)

func GuessingType(sub string) string {
	lsub := strings.ToLower(sub)
	if strings.Contains(lsub, strings.ToLower("[V4+ Styles]")) {
		return "ass"
	}
	if strings.Contains(lsub, strings.ToLower("[V4 Styles]")) {
		return "ssa"
	}
	if strings.HasPrefix(strings.TrimLeft(sub, " "), "WEBVTT") {
		return "vtt"
	}
	re := regexp.MustCompile(`(\d{1,2}):(\d{2}):(\d{2})[.,](\d{2,3})`)
	for _, l := range strings.Split(sub, "\n") {
		if len(re.FindAllString(l, 2+1)) == 2 {
			return "srt"
		}
	}
	return ""
}
