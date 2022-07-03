package unpack

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/mholt/archiver/v4"
)

func WalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	file, err := os.Open(packed)
	if err != nil {
		return err
	}
	defer file.Close()
	format, input, err := archiver.Identify(packed, file)
	if err == archiver.ErrNoMatch {
		var r *sevenzip.ReadCloser
		if strings.ToLower(filepath.Ext(packed)) == ".7z" {
			r, _ = sevenzip.OpenReader(packed)
		}
		if r != nil {
			for _, f := range r.File {
				if f.FileInfo().IsDir() {
					continue
				}
				rc, err := f.Open()
				if err != nil {
					continue
				}
				hook(rc, f.FileInfo())
				rc.Close()
			}
			r.Close()
		} else {
			file.Seek(0, 0)
			fl, err := os.Lstat(packed)
			if err != nil {
				return err
			}
			hook(file, fl)
		}
	} else if ex, ok := format.(archiver.Extractor); ok {
		ex.Extract(context.Background(), input, nil, func(ctx context.Context, f archiver.File) error {
			if f.IsDir() {
				return nil
			}
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			hook(rc, f.FileInfo)
			rc.Close()
			return nil
		})
	}
	return nil
}
