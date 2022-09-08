package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

/*
cache/<md5 key>/<content>
*/
const (
	cacheDir   = "cache"
	nameAccess = "last_access"
)

func updateAccess(dir string) {
	f, err := os.OpenFile(filepath.Join(dir, nameAccess), os.O_CREATE|os.O_WRONLY, 0755)
	if err == nil {
		f.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	}
}

func checkDir(d string) error {
	if d != cacheDir {
		if err := checkDir(filepath.Dir(d)); err != nil {
			return err
		}
	}
	info, err := os.Stat(d)
	if err != nil {
		return os.Mkdir(d, 0755)
	}
	if !info.IsDir() {
		os.Remove(d)
		return os.Mkdir(d, 0755)
	}
	return nil
}

// TODO: see if we need to replace string with interface{}

func md5Key(k string) string {
	hash := md5.Sum([]byte(k))
	return hex.EncodeToString(hash[:])
}

func MergeKeys(k ...string) string {
	return fmt.Sprintf("%v", k)
}

func UpdateKey(k string, sub string) {
	cacheDir := filepath.Join(cacheDir, sub)

	err := checkDir(cacheDir)
	if err != nil {
		return
	}

	fn := filepath.Join(cacheDir, md5Key(k))
	_, err = os.Stat(fn)
	if err != nil {
		os.Mkdir(fn, 0755)
	}
	os.Chtimes(fn, time.Now(), time.Now())
}

func StatKey(interval time.Duration, k string, sub string) bool {
	cacheDir := filepath.Join(cacheDir, sub)

	err := checkDir(cacheDir)
	if err != nil {
		return false
	}

	fn := filepath.Join(cacheDir, md5Key(k))
	s, err := os.Stat(fn)
	if err == nil {
		return time.Since(s.ModTime()) >= interval
	}
	return true
}

func TryGet(k string, sub string, or func() string) string {
	cacheDir := filepath.Join(cacheDir, sub)

	fn := filepath.Join(cacheDir, md5Key(k))
	err := checkDir(fn)

	var f *os.File
	if err == nil {
		f, err = os.Open(fn)
	}
	if err == nil {
		defer f.Close()
		var v []fs.DirEntry
		v, err = f.ReadDir(-1)
		if err == nil {
			for i := range v {
				if v[i].Name() != nameAccess {
					if sub == "references" {
						updateAccess(fn)
					}
					return filepath.Join(fn, v[0].Name())
				}
			}
		}
	}

	s := or()
	if s != "" {
		new := filepath.Join(fn, filepath.Base(s))
		err = moveFile(s, new)
		if err == nil {
			if sub == "references" {
				updateAccess(fn)
			}
			return new
		}
	}
	return s
}

// for docker
func moveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("Couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("Writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Failed removing original file: %s", err)
	}
	return nil
}
