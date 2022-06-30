package ffmpeg

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strconv"
)

var SubtitleCodecToFormat map[string]string = map[string]string{
	"hdmv_pgs_subtitle": "sup",
	//"dvd_subtitle": "microdvd",

	"subrip": "srt",
	"ass":    "ass",
	"webvtt": "srt",
}

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

func ExtractSubtitle(path string, info StreamInfo) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-v", "quiet", "-i", path, "-map", "0:"+strconv.Itoa(info.Index), "-c", "copy", "-f", SubtitleCodecToFormat[info.CodecName], "-")
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	err := cmd.Run()
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}
