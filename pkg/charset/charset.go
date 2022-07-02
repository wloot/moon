package charset

import (
	"bytes"
	"io"

	"github.com/gogs/chardet"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

func AnyToUTF8(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	charset, err := chardet.NewTextDetector().DetectBest(data)
	if err != nil {
		return []byte{}, err
	}
	if charset.Charset == "UTF-8" {
		return removeBOM(data), nil
	}

	encoding, err := ianaindex.MIB.Encoding(charset.Charset)
	if err != nil {
		return []byte{}, err
	}
	transformed, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), encoding.NewDecoder()))
	if err != nil {
		return []byte{}, err
	}

	return removeBOM(transformed), nil
}

func removeBOM(utf8 []byte) []byte {
	if utf8[0] == 0xef && utf8[1] == 0xbb && utf8[2] == 0xbf {
		utf8 = utf8[3:]
	}
	return utf8
}
