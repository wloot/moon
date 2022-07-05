package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"moon/pkg/cache"
	"moon/pkg/charset"
	"moon/pkg/emby"
	"moon/pkg/ffmpeg"
	"moon/pkg/ffsubsync"
	"moon/pkg/provider/zimuku"
	"moon/pkg/subtype"
	"moon/pkg/unpack"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/asticode/go-astisub"
	"github.com/otiai10/gosseract/v2"
)

type Subinfo struct {
	format  string
	data    []byte
	info    *astisub.Subtitles
	chinese bool
	double  bool
	tc      bool
}

var SETTNGS_videopath_map map[string]string = map[string]string{}

var SETTINGS_emby_url string = "http://play.charontv.com"
var SETTINGS_emby_key string = "fe1a0f6c143043e98a1f3099bfe0a3a8"
var SETTINGS_emby_importcount int = 200

func main() {
	client := gosseract.NewClient()
	defer client.Close()
	b, _ := base64.StdEncoding.DecodeString("Qk3aHwAAAAAAADYAAAAoAAAAZAAAABsAAAABABgAAAAAAKQfAAAAAQAAAAEAAAAAAAAAAAAA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzczc3Ntry2rrSupqymtry2rrSuyMjI5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcyMjItry2rrSupqymtry20dLR0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLStry2rrSupqymtry2rrSupqymtry2rrSu0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzczc3Ntry2rrSupqymtry2zM3M0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm1tbWpqymtry2rrSupqym1tbW3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubmkqOSAJkAAJkAAJkAAJkAAJkAgJqAzMzM5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS39/fgJqAAJkAAJkAAJkAAJkAhqCG1dbV0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcAJkAAJkAAJkAAJkAAJkAAJkAAJkAAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmkqOSAJkAAJkAAJkAAJkAc5lzpa6lzMzM5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSmaqZAJkAAJkAAJkAAJkAaY9prLas1dbV0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLSB5YHQIxAPYo9Q49DQIxAFJEUAJkAip+K0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6AJkAFZIVPYo9Q49DFZIVAJkAkKWQ3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmOYw5AJkAFpMWQIxAPYo9Q49DQIxAPYo95ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSB5YHQIxAPYo9Q49DQIxAFJEUAJkAw8XD0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcBpYGQ49DQIxAPYo9Q49DFZIVAJkAzM7M3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3NzcVYtV5ubm3Nzc0tLS5ubmeph6D5IPAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmgpuCD5IPf55/3Nzc0tLSf55/EJMQAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS1NXUOYw5Losuu8C73Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcVYtV5ubm3Nzc0tLS5ubmeph6D5IPiKGI3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmWY9Z0tLS5ubm3Nzc0tLSf55/EJMQfJV85ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmMI0wAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSAJkAMI0w0tLS5ubm3Nzc0tLSM48zAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSxsnGN403Ooo6v8O/3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmMI0wAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSM48zAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSM48zAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcAJkAM48z3Nzc0tLS5ubm3NzcLosuAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSub+5Mo0yRIlExsnG3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSM48zAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcLosuAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmmKeYGY8ZXZNd3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmAJkAGY8Zn66f3Nzc0tLSn66fGpAaAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSrreuLI0sS4pLztDO3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcLosuAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmMI0wAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSv8O/bpRuaY9pJpAmVY5VxsbG5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSAJkAAJkAI40jc5lzbpRuI40jAJkAYZFh0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSprGmJI4kUYpR2dnZ3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmz9DPgJeA5ubm3Nzc0tLSmaqZGJEYAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS2dnZhp2G0tLS5ubm3Nzci5yLGZIZAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzci5yLAJkAAJkAAJkAUZFRzs/O0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcAJkAG5IbT41PAJkAAJkAAJkAWItY2tra3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSTJBMAJkAVYtV5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSzM7MFpEWYI1gaZZpZJFkII0gAJkAaZNp0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6F5IXZJFkYI1gaZZpIY8hAJkAbpdu3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmvcC9epV6hqCGKo4qAJkAVZFV3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmAJkALosuxsnGgJqAepV6hqCGx8jH0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcl6OXHZEdAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6AJkAAJkAAJkAAJkAAJkAX4xf29vb3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmw8XDAJkAAJkAAJkAAJkAAJkAY5FjyMjI5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSrLasHpAeAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSAJkAEpISepV65ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcLosuAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmw8XDAJkAL48viZ+Jg5iDj6WPycrJ0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSzM7MAJkAK4srj6WPiZ+Jg5iD0tPS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSM48zAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcfJV8AJkASY1J0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmMI0wAJkA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSzM7MAJkARYpF5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6AJkASY1J0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3NzcTYpN5ubm3Nzc0tLS5ubmdJZ0DZMNAJkA3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmw8XDAJkAFpMWhZ2F0tLS5ubmYpFiwcLB5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSVZFV3Nzc0tLS5ubm3NzcbpBuDpQOAJkA0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6AJkASY1J0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmw8XDAJkATJBM3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubmBpYGNIo0OY85N403NIo0E5MTAJkAi5yL5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS39/fgJqAAJkAGZIZN403NIo0DJUMw8XD0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3NzcBZYFOY85N403NIo0OY85EpISAJkAmaqZ3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmw8XDAJkAE5MTN403NIo0OY85N403wMHA5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSzM7MAJkAEZIROY85N403NIo0OY85ycrJ0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLSoa+hAJkAAJkAAJkAAJkAAJkAjaSN1tbW0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS29vbdJZ0AJkAAJkAAJkAgZeB4ODg3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubmmqiaAJkAAJkAAJkAAJkAAJkAh56Hzc3N5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLSzM7MAJkAAJkAAJkAAJkAAJkAAJkAw8XD0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzcury6AJkAAJkAAJkAAJkAAJkAAJkAzM7M3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzczs7Ov8O/t7u3r7Ovv8O/t7u3ysrK5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS2dnZt7u3r7Ovv8O/09TT0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS4eHht7u3r7Ovv8O/t7u3r7Ov3d3d3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzczs7Ov8O/t7u3r7Ovv8O/t7u3r7Ov4eHh3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm19fXr7Ovv8O/t7u3r7Ovv8O/t7u3zs7O5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS5ubm3Nzc0tLS")
	client.SetImageFromBytes(b)
	t, e := client.Text()
	if e != nil {
		fmt.Printf("haha %v\n", e)
	}
	print(t, "\n")
	return

start:
	embyAPI := emby.New(SETTINGS_emby_url, SETTINGS_emby_key)
	zimuku := zimuku.New()

	var movieList []emby.EmbyVideo
	for i := 0; len(movieList) < SETTINGS_emby_importcount; i += 1 {
		ids := embyAPI.RecentMovie(SETTINGS_emby_importcount/2, i*SETTINGS_emby_importcount/2)
		for _, id := range ids {
			v := embyAPI.MovieInfo(id)
			if len(v.ProductionLocations) > 0 && v.ProductionLocations[0] == "China" {
				continue
			}
			if whatlanggo.Detect(v.OriginalTitle).Lang == whatlanggo.Cmn {
				continue
			}
			if v.MediaStreams[1].Type == "Audio" && v.MediaStreams[1].DisplayLanguage == "Chinese Simplified" {
				continue
			}
			movieList = append(movieList, v)
		}
	}

	for _, v := range movieList {
		var hasSub = false
		for _, stream := range v.MediaStreams {
			if stream.Type == "Subtitle" && stream.DisplayLanguage == "Chinese Simplified" {
				if stream.IsExternal == false {
					hasSub = true
					break
				}
				path := stream.Path[:len(stream.Path)-len(filepath.Ext(stream.Path))]
				// Emby 自带的字幕下载
				if strings.HasSuffix(path, ".zh-CN") == false {
					hasSub = true
					break
				}
			}
		}
		interval := time.Hour * 24 * 7
		if hasSub == true {
			interval = time.Hour * 24 * 30
			if time.Now().Sub(v.GetPremiereDate()) > 180*time.Hour*24 {
				interval = time.Hour * 24 * 180
			}
		}
		if time.Now().Sub(v.GetDateCreated()) <= time.Hour*24*3 || time.Now().Sub(v.GetPremiereDate()) <= time.Hour*24*30 {
			interval = time.Hour * 24
		}
		ok, err := cache.StatKey(interval, v.Path)
		if !ok || err != nil {
			if err != nil {
				fmt.Printf("cache dir may wrong: %v\n", err)
			}
			continue
		}

		if v.OriginalTitle == v.Name {
			embyAPI.Refresh(v.Id, true)
			time.Sleep(30 * time.Second)
			v = embyAPI.MovieInfo(v.Id)
		}
		for old, new := range SETTNGS_videopath_map {
			if strings.HasPrefix(v.Path, old) {
				v.Path = new + v.Path[len(old):]
			}
		}

		subFiles, failed := zimuku.SearchMovie(v)
		if failed == true {
			cache.DelEmpty(v.Path)
		}
		var subSorted []Subinfo
		for _, subName := range subFiles {
			err := unpack.WalkUnpacked(subName, func(reader io.Reader, info fs.FileInfo) {
				name := info.Name()
				t := strings.ToLower(filepath.Ext(name))
				if len(t) > 0 {
					t = t[1:]
				}
				data, _ := io.ReadAll(reader)
				if transformed, err := charset.AnyToUTF8(data); err == nil {
					data = transformed
				}
				if len(data) == 0 {
					fmt.Printf("ignoring empty sub %v\n", name)
					return
				}

				readSub := func(data []byte, ext string) (*astisub.Subtitles, error) {
					var s *astisub.Subtitles
					var err error
					if ext == "ssa" || ext == "ass" {
						// 一个常见的字幕typo
						data = bytes.Replace(data, []byte(",&H00H202020,"), []byte(",&H00202020,"), 1)
						data = bytes.Replace(data, []byte("[Aegisub Project Garbage]"), []byte(""), 1)
						s, err = astisub.ReadFromSSA(bytes.NewReader(data))
					}
					if ext == "srt" {
						s, err = astisub.ReadFromSRT(bytes.NewReader(data))
					}
					if ext == "vtt" {
						s, err = astisub.ReadFromWebVTT(bytes.NewReader(data))
					}
					return s, err
				}
				s, err := readSub(data, t)
				if s == nil || err != nil || len(s.Items) == 0 {
					t = subtype.GuessingType(string(data))
					s, err = readSub(data, t)
					if err != nil || s == nil || len(s.Items) == 0 {
						fmt.Printf("ignoring sub %v as err %v or guessed type '%v'\n", name, err, t)
						return
					}
				}

				if t == "vtt" {
					var buf bytes.Buffer
					s.WriteToSRT(&buf)
					data = buf.Bytes()
					t = "srt"
				}
				subSorted = append(subSorted, Subinfo{
					data:   data,
					info:   s,
					format: t,
				})
			})
			if err != nil {
				fmt.Printf("open sub file %v faild: %v\n", subName, err)
			}
		}

		jianfan := charset.NewJianfan()
		for i := range subSorted {
			countTC := 0
			countChars := 0
			countCh := 0
			countLines := 0
			countAllLines := 0
			durationLast := make(map[string]struct{})
			for _, v := range subSorted[i].info.Items {
				if len(v.Lines) == 0 {
					continue
				}
				countAllLines += len(v.Lines)

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

			if countLines/2 < countCh {
				subSorted[i].chinese = true
			}
			if countLines*3 < countAllLines*2 {
				subSorted[i].double = true
			}
			if countChars/10 <= countTC {
				subSorted[i].tc = true
			}
		}
		for i := len(subSorted) - 1; i >= 0; i-- {
			need := true
			if subSorted[i].chinese == false {
				need = false
			}
			if subSorted[i].tc == true {
				need = false
			}
			if need == false {
				subSorted = append(subSorted[:i], subSorted[i+1:]...)
			}
		}

		if len(subSorted) == 0 {
			fmt.Printf("total sub downloaded is 0\n")
			continue
		}
		sort.Slice(subSorted, func(i, j int) bool {
			if subSorted[i].double != subSorted[j].double {
				return subSorted[i].double == true
			}
			return false
		})

		selectedSub := subSorted[0]
		backupType := "srt"
		if selectedSub.format == "srt" {
			backupType = "ass"
		}
		var reference string
	savesub:
		name := v.Path[:len(v.Path)-len(filepath.Ext(v.Path))] + ".chs." + selectedSub.format
		err = os.WriteFile(name, selectedSub.data, 0644)
		if err != nil {
			fmt.Printf("failed to write sub file: %v\n", err)
			continue
		}
		fmt.Printf("sub written to %v\n", name)
		if reference == "" && backupType != "" {
			reference = cache.TryGet(v.Path, func() string {
				reference := ffsubsync.FindBestReferenceSub(v)
				if reference == "" {
					fmt.Printf("no fit inter sub so extract audio for sync\n")
					reference, _ = ffmpeg.KeepAudio(v.Path)
				}
				return reference
			})
		}
		if reference == "" {
			ffsubsync.DoSync(name, v.Path, false)
		} else {
			ffsubsync.DoSync(name, reference, true)
		}
		if backupType != "" {
			for i := range subSorted {
				if subSorted[i].format == backupType {
					backupType = ""
					selectedSub = subSorted[i]
					goto savesub
				}
			}
		}
		embyAPI.Refresh(v.Id, false)
	}
	time.Sleep(6 * time.Hour)
	goto start
}
