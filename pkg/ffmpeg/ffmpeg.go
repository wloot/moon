package ffmpeg

import (
	"bytes"
	"encoding/json"
	"os/exec"
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
	const args = "-v quiet -print_format json -show_streams"
	argExec := strings.Fields(args)
	argExec = append(argExec, path)
	cmd := exec.Command("ffprobe", argExec...)
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
