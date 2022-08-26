package charset

import (
	"bytes"
	"errors"
	"io"

	"github.com/gogs/chardet"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

var (
	utf8Bom = []byte{0xEF, 0xBB, 0xBF}
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
		return bytes.TrimPrefix(data, utf8Bom), nil
	}

	encoding, err := ianaindex.MIB.Encoding(charset.Charset)
	if err != nil {
		return []byte{}, err
	}
	if encoding == nil {
		return []byte{}, errors.New("No such charset encoding")
	}
	transformed, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), encoding.NewDecoder()))
	if err != nil {
		return []byte{}, err
	}

	return bytes.TrimPrefix(transformed, utf8Bom), nil
}
