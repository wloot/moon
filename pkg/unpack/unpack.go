package unpack

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/gen2brain/go-unarr"
	"github.com/mholt/archiver/v4"
)

type unarrFileInfo struct {
	a *unarr.Archive
}

func (u unarrFileInfo) Name() string {
	return u.a.Name()
}
func (u unarrFileInfo) Size() int64 {
	return int64(u.a.Size())
}
func (u unarrFileInfo) Mode() fs.FileMode {
	return fs.FileMode(0)
}
func (u unarrFileInfo) ModTime() time.Time {
	return u.a.ModTime()
}
func (u unarrFileInfo) IsDir() bool {
	return false
}
func (u unarrFileInfo) Sys() any {
	return nil
}

func WalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	file, err := os.Open(packed)
	if err != nil {
		return err
	}
	defer file.Close()
	// CGO: start
	a, err := unarr.NewArchiveFromReader(file)
	if err == nil {
		for {
			err := a.Entry()
			if err != nil {
				if err == io.EOF {
					break
				}
				continue
			}
			hook(a, unarrFileInfo{a: a})
		}
		a.Close()
		return nil
	}
	file.Seek(0, 0)
	// CGO: end
	// Golang native that could have many panics and errors
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
