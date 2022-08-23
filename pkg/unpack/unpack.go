package unpack

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/bodgit/sevenzip"
	"github.com/mholt/archiver/v4"
)

func WalkUnpacked(packed string, hook func(io.Reader, fs.FileInfo)) error {
	fmt.Printf("c1\n")
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
	fmt.Printf("c2\n")
	format, input, err := archiver.Identify("", file)
	fmt.Printf("c3\n")
	if err == archiver.ErrNoMatch {
		fmt.Printf("c4\n")
		r, err := sevenzip.OpenReader(packed)
		fmt.Printf("c5\n")
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
			fmt.Printf("c8\n")
			file.Seek(0, 0)
			fmt.Printf("c9\n")
			fl, err := os.Lstat(packed)
			fmt.Printf("c10\n")
			if err != nil {
				return err
			}
			hook(file, fl)
		}
	} else if ex, ok := format.(archiver.Extractor); ok {
		fmt.Printf("c6\n")
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
		fmt.Printf("c7\n")
	}
	return nil
}
