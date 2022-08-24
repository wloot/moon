package charset

import (
	"bytes"
	"errors"
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
	if charset.Confidence < 25 {
		return []byte{}, errors.New("No confidence")
	}
	if charset.Charset == "UTF-8" {
		return data, nil
	}

	encoding, err := ianaindex.MIB.Encoding(charset.Charset)
	if err != nil {
		return []byte{}, err
	}
	if encoding == nil {
		return []byte{}, errors.New("Charset detection error")
	}
	transformed, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), encoding.NewDecoder()))
	if err != nil {
		return []byte{}, err
	}

	return transformed, nil
}

func RemoveBom(b []byte) []byte {
	var (
		utf8Bom    = []byte{0xEF, 0xBB, 0xBF}
		utf16beBom = []byte{0xFE, 0xFF}
		utf16leBom = []byte{0xFF, 0xFE}
		utf32beBom = []byte{0x00, 0x00, 0xFE, 0xFF}
		utf32leBom = []byte{0xFF, 0xFE, 0x00, 0x00}
	)
	if bytes.HasPrefix(b, utf8Bom) {
		return bytes.TrimPrefix(b, utf8Bom)
	}
	if bytes.HasPrefix(b, utf16beBom) {
		return bytes.TrimPrefix(b, utf16beBom)
	}
	if bytes.HasPrefix(b, utf16leBom) {
		return bytes.TrimPrefix(b, utf16leBom)
	}
	if bytes.HasPrefix(b, utf32beBom) {
		return bytes.TrimPrefix(b, utf32beBom)
	}
	if bytes.HasPrefix(b, utf32leBom) {
		return bytes.TrimPrefix(b, utf32leBom)
	}
	return b
}
