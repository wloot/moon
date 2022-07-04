package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
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
		if err := checkDir(cacheDir); err != nil {
			return err
		}
	}
	info, err := os.Stat(d)
	if err != nil {
		return os.Mkdir(d, 0755)
	}
	if info.IsDir() == false {
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

func StatKey(interval time.Duration, k string) (bool, error) {
	err := checkDir(cacheDir)
	if err != nil {
		return false, err
	}

	fn := filepath.Join(cacheDir, md5Key(k))
	s, err := os.Stat(fn)
	if err != nil {
		err := os.Mkdir(fn, 0755)
		if err == nil {
			return true, nil
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
			fmt.Printf("cache: got %v\n", v[0].Name())
			return filepath.Join(fn, v[0].Name())
		}
	}

	s := or()
	if s != "" {
		new := filepath.Join(fn, filepath.Base(s))
		err = moveFile(s, new)
		if err == nil {
			return new
		}
		fmt.Printf("cache: save failed %v\n", err)
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
