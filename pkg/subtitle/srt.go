package subtitle

import (
	"bytes"
	"moon/pkg/charset"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
)

func ReplaceSpecString(inString string, rep string) string {
	return regexp.MustCompile(`(?m)[\p{P}|\p{Z}}}|\p{S}\s|\t|\v]`).ReplaceAllString(inString, rep)
}

type Srt struct {
}

func (p Srt) GetParserName() string {
	return "srt"
}

// DetermineFileTypeFromBytes 确定字幕文件的类型，是双语字幕或者某一种语言等等信息
func (p Srt) DetermineFileTypeFromBytes(inBytes []byte, filename string) bool {
	var err error
	var s *astisub.Subtitles
	switch filepath.Ext(strings.ToLower(filename)) {
	case ".srt":
		s, err = astisub.ReadFromSRT(bytes.NewReader(inBytes))
	case ".ssa", ".ass":
		s, err = astisub.ReadFromSSA(bytes.NewReader(inBytes))
	default:
		return false
	}
	if err != nil {
		return false
	}

	countLines := 0
	countCh := 0

	countTC := 0
	countChars := 0
	jianfan := charset.NewJianfan()

	durationLast := make(map[string]struct{})

	for _, v := range s.Items {
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

		print(line, "\n")

		countLines += 1
		// lingua too slow
		lang := whatlanggo.Detect(line)
		if lang.Lang == whatlanggo.Cmn {
			countCh += 1

			countChars += len([]rune(line))
			countTC += jianfan.CountCht(line)
		}
	}

	if countLines/2 > countCh {
		print("a")
		return false
	}
	if countChars/10 <= countTC {
		print("b")
		return false
	}

	return true

	/*orgDialogues := p.parseContent(inBytes)
	if len(orgDialogues) <= 0 {
		return false
	}
	for _, oneDialogue := range orgDialogues {
		if len(oneDialogue.Lines) == 0 {
			continue
		}

		fixedLine := oneDialogue.Lines[0]
		// 剔除 {\fn微软雅黑\fs14}C'mon, Rick. We're -- We're almost there. {} 这一段
		fixedLine = regexp.MustCompile(`(?m)((?i){[^}]*})`).ReplaceAllString(fixedLine, "")
		fixedLine = regexp.MustCompile(`(?m)((?i)\[[^]]*\])`).ReplaceAllString(fixedLine, "")
		fixedLine = strings.ReplaceAll(fixedLine, `\N`, "")

		if fixedLine != "" {
			countLines += 1
			lang, _ := langDetector.DetectLanguageOf(fixedLine)
			if lang == lingua.Chinese {
				countCh += 1

				countChars += len([]rune(fixedLine))
				countTC += jianfan.CountCht(fixedLine)
			}
		}
	}*/
}

func (p Srt) parseContent(inBytes []byte) []OneDialogue {
	allString := string(inBytes)

	// 注意，需要替换掉 \r 不然正则表达式会有问题
	allString = strings.ReplaceAll(allString, "\r", "")
	lines := strings.Split(allString, "\n")
	// 需要把每一行如果是多余的特殊剔除掉
	// 这里的目标是后续的匹配更加容易，但是，后续也得注意
	// 因为这个样的操作，那么匹配对白内容的时候，可能是不存在的，只要是 index 和 时间匹配上了，就应该算一句话，只要在 dialogue 上是没得问题的
	// 而 dialogueFilter 中则可以把这样没有内容的排除，但是实际时间轴匹配的时候还是用 dialogue 而不是 dialogueFilter
	filterLines := make([]string, 0)
	for _, line := range lines {
		// 如果当前的这一句话，为空，或者进过正则表达式剔除特殊字符后为空，则跳过
		if ReplaceSpecString(line, "") == "" {
			continue
		}
		filterLines = append(filterLines, line)
	}

	dialogues := make([]OneDialogue, 0)
	/*
		这里可以确定，srt 格式，开始一定是第一句话，那么首先就需要找到，第一行，一定是数字的，从这里开始算起
		1. 先将 content 进行 \r 的替换为空
		2. 将 content 进行 \n 来分割
		3. 将分割的数组进行筛选，把空行剔除掉
		4. 然后使用循环，用下面的 steps 进行解析一句对白
		steps:
				0	找对白的 ID
				1	找时间轴
				2	找对白内容，可能有多行，停止的方式，一个是向后能找到 0以及2 或者 是最后一行
	*/
	steps := 0
	nowDialogue := NewOneDialogue()
	newOneDialogueFun := func() {
		// 重新新建一个缓存对白，从新开始
		steps = 0
		nowDialogue = NewOneDialogue()
	}
	// 使用过滤后的列表
	for i, line := range filterLines {

		if steps == 0 {
			// 匹配对白的索引
			line = ReplaceSpecString(line, "")
			dialogueIndex, err := strconv.Atoi(line)
			if err != nil {
				newOneDialogueFun()
				continue
			}
			nowDialogue.Index = dialogueIndex
			// 继续
			steps = 1
			continue
		}

		if steps == 1 {
			// 匹配时间
			matched := regexp.MustCompile(`([\d:,]+)\s+-{2}\>\s+([\d:,]+)`).FindAllStringSubmatch(line, -1)
			if matched == nil || len(matched) < 1 {
				matched = regexp.MustCompile(`([\d:.]+)\s+-{2}\>\s+([\d:.]+)`).FindAllStringSubmatch(line, -1)
				if matched == nil || len(matched) < 1 {
					newOneDialogueFun()
					continue
				}
			}
			nowDialogue.StartTime = matched[0][1]
			nowDialogue.EndTime = matched[0][2]

			// 是否到结尾
			if i+1 > len(filterLines)-1 {
				// 是尾部
				// 那么这一个对白就需要 add 到总列表中了
				dialogues = append(dialogues, nowDialogue)
				newOneDialogueFun()
				continue
			}
			// 如上面提到的，因为把特殊字符的行去除了，那么一个对话，如果只有 index 和 时间，也是需要添加进去的
			if p.needMatchNextContentLine(filterLines, i+1) == true {
				// 是，那么也认为当前这个对话完成了，需要 add 到总列表中了
				dialogues = append(dialogues, nowDialogue)
				newOneDialogueFun()
				continue
			}
			// 非上述特殊情况，继续
			steps = 2
			continue
		}

		if steps == 2 {
			// 在上述情况排除后，才继续
			// 匹配内容

			if len(regexp.MustCompile(`(?m)([1-9]\d*\.?\d*)|(0\.\d*[1-9])`).FindAllString(line, -1)) > 5 {
				continue
			}

			nowDialogue.Lines = append(nowDialogue.Lines, line)
			// 是否到结尾
			if i+1 > len(filterLines)-1 {
				// 是尾部
				// 那么这一个对白就需要 add 到总列表中了
				dialogues = append(dialogues, nowDialogue)
				newOneDialogueFun()
				continue
			}

			// 不是尾部，那么就需要往后看两句话，是否是下一个对白的头部（index 和 时间）
			if p.needMatchNextContentLine(filterLines, i+1) == true {
				// 是，那么也认为当前这个对话完成了，需要 add 到总列表中了
				dialogues = append(dialogues, nowDialogue)
				newOneDialogueFun()
				continue
			} else {
				// 如果还不是，那么就可能是这个对白有多行，有可能是同一种语言的多行，也可能是多语言的多行
				// 那么 step 应该不变继续是 2
				continue
			}
		}
	}

	return dialogues
}

// needMatchNextContentLine 是否需要继续匹配下一句话作为一个对白的对话内容
func (p Srt) needMatchNextContentLine(lines []string, index int) bool {
	if index+1 > len(lines)-1 {
		return false
	}

	// 匹配到对白的 Index
	_, err := strconv.Atoi(lines[index])
	if err != nil {
		return false
	}
	// 匹配到字幕的时间
	matched := regexp.MustCompile(`([\d:,]+)\s+-{2}\>\s+([\d:,]+)`).FindAllStringSubmatch(lines[index+1], -1)
	if matched == nil || len(matched) < 1 {
		matched = regexp.MustCompile(`([\d:.]+)\s+-{2}\>\s+([\d:.]+)`).FindAllStringSubmatch(lines[index+1], -1)
		if matched == nil || len(matched) < 1 {
			return false
		}
	}

	return true
}
