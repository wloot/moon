package subtitle

type Subinfo struct {
	PrefixDialogueString string        // 在 Dialogue: 这个关键词之前的字符串，ass 中的字体以及其他信息的描述
	Content              string        // 字幕的内容
	FromWhereSite        string        // 从那个网站下载的
	Name                 string        // 字幕的名称，注意，这里需要额外的赋值，不会自动检测
	Ext                  string        // 字幕的后缀名
	Lang                 string        // 识别出来的语言
	FileFullPath         string        // 字幕文件的全路径
	Data                 []byte        // 字幕的二进制文件内容
	Dialogues            []OneDialogue // 整个字幕文件的所有对话，如果是做时间轴匹配，就使用原始的
	DialoguesFilter      []OneDialogue // 整个字幕文件的所有对话，过滤掉特殊字符的对白
	DialoguesFilterEx    []OneDialogue // 整个字幕文件的所有对话，过滤掉特殊字符的对白，这里会把一句话中支持的 中、英、韩、日 四国语言给分离出来
	CHLines              []string      // 抽取出所有的中文对话
	OtherLines           []string      // 抽取出所有的第二语言对话，可能是英文、韩文、日文
}

type OneDialogue struct {
	Index     int      // 对白的索引
	StartTime string   // 开始时间
	EndTime   string   // 结束时间
	StyleName string   // StyleName
	Lines     []string // 台词
}

func NewOneDialogue() OneDialogue {
	return OneDialogue{
		Lines: make([]string, 0),
	}
}
