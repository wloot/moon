package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

/*

cache/<md5 key>/content

*/
var cacheDir = "cache"

func checkDir(d string) error {
	if d != cacheDir {
		err := checkDir(cacheDir)
		if err != nil {
			return err
		}
	}
	info, err := os.Stat(d)
	if err != nil {
		if err == os.ErrNotExist {
			return os.Mkdir(cacheDir, 0755)
		}
		return err
	}
	if info.IsDir() == false {
		err := os.Remove(cacheDir)
		if err == nil {
			return os.Mkdir(cacheDir, 0755)
		}
		return err
	}
	return nil
}

func md5Key(k string) string {
	hash := md5.Sum([]byte(k))
	return hex.EncodeToString(hash[:])
}

func MergeKeys(k ...interface{}) string {
	return fmt.Sprintf("%v", k)
}

func StatKey(interval time.Duration, k string) (bool, error) {
	err := checkDir(cacheDir)
	if err != nil {
		return false, err
	}

	fn := filepath.Join(cacheDir, md5Key(k))
	s, err := os.Stat(fn)
	if err != nil {
		if err == os.ErrNotExist {
			_, err = os.Create(fn)
			if err == nil {
				return true, nil
			}
		}
		return false, err
	}
	if time.Now().Sub(s.ModTime()) >= interval {
		err := os.Chtimes(fn, time.Now(), time.Now())
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func TryGet(k string, or func() string) string {
	fn := filepath.Join(cacheDir, md5Key(k))
	err := checkDir(fn)

	var f *os.File
	if err == nil {
		f, err = os.Open(fn)
		defer f.Close()
	}
	if err == nil {
		var v []fs.DirEntry
		v, err = f.ReadDir(-1)
		if err == nil && len(v) > 0 {
			return filepath.Join(fn, v[0].Name())
		}
	}

	s := or()
	if s != "" {
		new := filepath.Join(fn, filepath.Base(s))
		err = os.Rename(s, new)
		if err == nil {
			return new
		}
	}
	return s
}
