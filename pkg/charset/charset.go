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
		return data, nil
	}

	encoding, err := ianaindex.MIB.Encoding(charset.Charset)
	if err != nil {
		return []byte{}, err
	}
	transformed, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), encoding.NewDecoder()))
	if err != nil {
		return []byte{}, err
	}
	if transformed[0] == 0xef && transformed[1] == 0xbb && transformed[2] == 0xbf {
		transformed = transformed[3:]
	}

	return transformed, nil
}
