package pgstosrt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// Port https://github.com/boban-bmw/pgs-to-srt-ffsubsync

const (
	jumpPG          int = 2
	jumpPTS         int = 4
	jumpDTS         int = 4
	jumpSegmentType int = 1
	jumpSegmentSize int = 2
)

var segmentEnd = []byte{80, 00}

func PgsToSrt(pgs []byte) []byte {
	ts := generateTimestamps(pgs)
	srt := generateSrt(ts)
	var output string
	for _, s := range srt {
		output += s
	}
	return []byte(output)
}

func generateTimestamps(pgs []byte) []int {
	var output []int
	i := 0
	for i < len(pgs) {
		i += jumpPG

		timestampBuffer := pgs[i : i+jumpPTS]
		i += jumpPTS

		i += jumpDTS

		segmentType := pgs[i : i+jumpSegmentType]
		i += jumpSegmentType

		segmentSize := pgs[i : i+jumpSegmentSize]
		i += jumpSegmentSize

		i += int(binary.BigEndian.Uint16(segmentSize))

		if bytes.Equal(segmentType, segmentEnd) {
			output = append(output, int(binary.BigEndian.Uint32(timestampBuffer))/90)
		}
	}
	return output
}

func generateSrt(timstamps []int) []string {
	if len(timstamps)%2 != 0 {
		timstamps = timstamps[:len(timstamps)-1]
	}

	var lines []string

	counter := 1
	for i := 0; i < len(timstamps); i += 2 {
		lines = append(lines, strconv.Itoa(counter)+"\n")
		lines = append(lines, timestampToSrt(timstamps[i])+" --> "+timestampToSrt(timstamps[i+1])+"\n")

		lines = append(lines, strconv.Itoa(counter)+"\n")
		lines = append(lines, "\n")

		counter += 1
	}
	return lines
}

func timestampToSrt(timestamp int) string {
	milliseconds := fmt.Sprintf("%03d", timestamp%1000)

	timestampInSeconds := int(math.Floor(float64(timestamp) / 1000))
	seconds := fmt.Sprintf("%02d", timestampInSeconds%60)

	timestampInMinutes := int(math.Floor(float64(timestampInSeconds) / 60))
	minutes := fmt.Sprintf("%02d", timestampInMinutes%60)

	timestampInHours := int(math.Floor(float64(timestampInMinutes) / 60))
	hours := fmt.Sprintf("%02d", timestampInHours%60)

	return hours + ":" + minutes + ":" + seconds + "," + milliseconds
}
