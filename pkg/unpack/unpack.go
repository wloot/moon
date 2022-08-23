package unpack

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bodgit/sevenzip"
	"github.com/mholt/archiver/v4"
)

func WalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	// https://github.com/mholt/archiver/issues/345
	if filepath.Base(packed) == "[zmk.pw]Batman.Forever.1995.BluRay.720p.DTS.2Audio.x264-CHD.rar" {
		return nil
	}
	file, err := os.Open(packed)
	if err != nil {
		return err
	}
	defer file.Close()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("walkUnpacked: catch panic: %v\n", r)
		}
	}()
	format, input, err := archiver.Identify("", file)
	if err == archiver.ErrNoMatch {
		r, err := sevenzip.OpenReader(packed)
		if err == nil {
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
		ex.Extract(context.Background(), input, nil, func(_ context.Context, f archiver.File) error {
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
