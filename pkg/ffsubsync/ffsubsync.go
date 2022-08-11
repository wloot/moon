package ffsubsync

import (
	"fmt"
	"moon/pkg/emby"
	"moon/pkg/ffmpeg"
	"moon/pkg/pgstosrt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

func FindBestReferenceSub(v emby.EmbyVideo) string {
	var extSub string
	streams := make([]emby.EmbyVideoStream, len(v.MediaStreams))
	copy(streams, v.MediaStreams)
	for i := len(streams) - 1; i >= 0; i-- {
		ok := streams[i].Type == "Subtitle" && !streams[i].IsExternal
		if ok {
			_, format := ffmpeg.SubtitleBestExtractFormat(streams[i].SubtitleCodecToFfmpeg())
			ok = format != ""
		}
		if !ok {
			streams = append(streams[:i], streams[i+1:]...)
		}
	}
	if len(streams) > 0 {
		bestSub := streams[0]
		for i := len(streams) - 1; i >= 0; i-- {
			if regexp.MustCompile(`\bSDH\b`).MatchString(streams[i].Title) {
				streams = append(streams[:i], streams[i+1:]...)
			}
		}
		if len(streams) > 0 {
			bestSub = streams[0]
		}
		for i := len(streams) - 1; i >= 0; i-- {
			if streams[i].IsForced {
				streams = append(streams[:i], streams[i+1:]...)
			}
		}
		if len(streams) > 0 {
			bestSub = streams[0]
		}
		for i := len(streams) - 1; i >= 0; i-- {
			if streams[i].Codec == "PGSSUB" {
				streams = append(streams[:i], streams[i+1:]...)
			}
		}
		if len(streams) > 0 {
			bestSub = streams[0]
		}
		fmt.Printf("extract inter sub for sync: %v\n", bestSub)
		subData, err := ffmpeg.ExtractSubtitle(v.Path, bestSub.Index, bestSub.SubtitleCodecToFfmpeg())
		if err == nil {
			_, ext := ffmpeg.SubtitleBestExtractFormat(bestSub.SubtitleCodecToFfmpeg())
			if ext == "sup" {
				subData = pgstosrt.PgsToSrt(subData)
				ext += ".srt"
			}
			name := filepath.Base(v.Path)
			name = name[:len(name)-len(filepath.Ext(name))] + "." + strconv.FormatInt(int64(bestSub.Index), 10) + "." + ext
			name = filepath.Join(os.TempDir(), name)
			err = os.WriteFile(name, subData, 0644)
			if err == nil {
				extSub = name
			}
		}
	}
	return extSub
}

func Sync(path string, reference string, isSub bool) {
	_, err := exec.LookPath("ffsubsync")
	if err != nil {
		return
	}
	cmdArg := []string{reference, "-i", path, "--overwrite-input"}
	if !isSub {
		cmdArg = append(cmdArg, "--reference-stream", "a:0")
	}
	cmd := exec.Command("ffsubsync", cmdArg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	cmd.Run()
}
