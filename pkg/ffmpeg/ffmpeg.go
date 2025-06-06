package ffmpeg

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type StreamInfo struct {
	Index     int    `json:"index"`
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Tags      struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type ffprobeInfo struct {
	Streams []StreamInfo `json:"streams"`
}

func ProbeVideo(path string) ([]StreamInfo, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", path)
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	err := cmd.Run()
	if err != nil {
		return []StreamInfo{}, err
	}
	var r ffprobeInfo
	err = json.Unmarshal(buf.Bytes(), &r)
	if err != nil {
		return []StreamInfo{}, err
	}
	return r.Streams, nil
}

func ExtractSubtitle(path string, index int, codec string) ([]byte, error) {
	c, f := SubtitleBestExtractFormat(codec)
	cmd := exec.Command("ffmpeg",
		"-v", "quiet", "-i", path, "-map", "0:"+strconv.Itoa(index), "-c", c, "-f", f, "-")
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	err := cmd.Run()
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

// TODO: 需要解析vobsub
func ExtractSubdvd(path string, index int) (string, error) {
	output := filepath.Join(os.TempDir(), filepath.Base(path)[:len(filepath.Base(path))-len(filepath.Ext(path))]+"."+strconv.Itoa(index))
	if strings.ToLower(filepath.Ext(path)) != ".mkv" {
		path = output + ".mkv"
		file, err := os.Create(path)
		if err != nil {
			return "", err
		}
		file.Close()
		defer os.Remove(path)
		cmd := exec.Command("ffmpeg", "-v", "quiet", "-i", path, "-map", "0:"+strconv.Itoa(index), "-c", "dvdsub", "-f", "matroska", path)
		err = cmd.Run()
		if err != nil {
			return "", err
		}
		index = 0
	}
	cmd := exec.Command("mkvextract", "tracks", filepath.Base(path), strconv.Itoa(index)+":"+output)
	cmd.Dir = filepath.Dir(path)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return output + ".sub", nil
}

func KeepAudio(path string) (string, error) {
	name := filepath.Base(path)
	name = name[:len(name)-len(filepath.Ext(name))] + ".1" + filepath.Ext(name)
	name = filepath.Join(os.TempDir(), name)
	cmd := exec.Command("ffmpeg", "-v", "quiet", "-i", path, "-map", "0:a:0", "-c", "copy", name)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return name, nil
}

func SubtitleBestExtractFormat(c string) (codec string, format string) {
	if c == "ass" {
		return "copy", "ass"
	}
	if c == "subrip" {
		return "copy", "srt"
	}
	if c == "webvtt" {
		return "subrip", "srt"
	}
	if c == "mov_text" {
		return "subrip", "srt"
	}
	if c == "hdmv_pgs_subtitle" {
		return "copy", "sup"
	}
	return "", ""
}
