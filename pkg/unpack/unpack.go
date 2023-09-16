package unpack

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/go-unarr"
	"github.com/mholt/archiver/v4"
)

type unarrFileInfo struct {
	a *unarr.Archive
}

func (u unarrFileInfo) Name() string {
	return filepath.Base(u.a.Name())
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

func unarrWalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	// CGO: start
	a, err := unarr.NewArchive(packed)
	if err != nil {
		return err
	}
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
	// CGO: end
}

func WalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	//defer func() {
	//	if r := recover(); r != nil {
	//		fmt.Printf("walkUnpacked: catch panic: %v\n", r)
	//	}
	//}()
	file, err := os.Open(packed)
	if err != nil {
		return err
	}
	defer file.Close()
	format, input, err := archiver.Identify("", file)
	if err == archiver.ErrNoMatch {
		file.Seek(0, 0)
		fl, err := os.Lstat(packed)
		if err != nil {
			return err
		}
		hook(file, fl)
		return nil
	} else if err == nil {
		// https://github.com/mholt/archiver/issues/345
		if format.Name() == ".rar" {
			err := unarrWalkUnpacked(packed, hook)
			if err == nil {
				return nil
			}
		}
		if tar, ok := format.(archiver.Tar); ok {
			tar.ContinueOnError = true
		}
		ex, _ := format.(archiver.Extractor)
		err = ex.Extract(context.Background(), input, nil, func(_ context.Context, f archiver.File) error {
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
		if err != nil && format.Name() != ".rar" {
			err = unarrWalkUnpacked(packed, hook)
		}
	}
	return err
}

func ZipRead(reader io.Reader, info fs.FileInfo) ([]byte, error) {
	data := make([]byte, 0, int(info.Size()))
	for len(data) != cap(data) {
		n, err := reader.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err != io.EOF {
				return data, err
			}
			// not work for unarr
			if len(data) != cap(data) {
				return data, errors.New("file size too small")
			}
			break
		}
	}
	zeros := len(data) > 0
	for _, e := range data {
		if e != byte(0) {
			zeros = false
			break
		}
	}
	if zeros {
		return data, errors.New("file seems to broke as all zeros")
	}
	return data, nil
}
