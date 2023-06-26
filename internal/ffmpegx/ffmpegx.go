package ffmpegx

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/StevenZack/tools/cmdToolkit"
	"github.com/StevenZack/tools/strToolkit"
	"github.com/StevenZack/transcoder/internal/tools"
)

type (
	MediaInfo struct {
		Width, Height   int
		DurationSeconds int
	}
)

const (
	// av1
	MAX_AV1_CONSTRAINT = 256
	// hevc
	MAX_HEVC_CONSTRAINT = 640
)

func FitConstraint(widthConstraint, heightConstraint int, w, h int) (int, int) {
	if w < widthConstraint && h < heightConstraint {
		return w, h
	}
	rh := int(float64(widthConstraint) / float64(w) * float64(h))
	if rh > heightConstraint {
		rw := int(float64(heightConstraint) / float64(h) * float64(w))
		return rw, heightConstraint
	}

	return widthConstraint, rh
}

// ffmpeg -i l.jpg -q:v 31 a.webp
func CompressImage(dst, filename string) error {
	_, e := cmdToolkit.Run("ffmpeg", "-y", "-i", filename, "-q:v", "31", dst)
	return e
}

// ffmpeg -i a.mp4 -ss 00:00:15 -frames:v 1 cover.webp
func CreateCoverOfVideo(dst, filename string, w, h int) error {
	_, e := cmdToolkit.Run("ffmpeg", "-y", "-i", filename, "-vf", fmt.Sprintf("scale=%dx%d", w, h), "-frames:v", "1", "-q:v", "31", dst)
	return e
}

/*
* ffmpeg -y -i a.mp4 -c:v libaom-av1 -vf scale=256x144,fps=10 -c:a aac -ac 1 -b:a 24k  -crf 42 -b:v 0 a.av1.mp4  -progress progress.txt &&
ffmpeg -y -i a.mp4 -c:v libx265 -vf scale=640x360,fps=10 -c:a aac -ac 1 -b:a 24k  -crf 42 -b:v 0 a.hevc.mp4 -progress progress.txt
*/
func CompressToAV1_HEVC(dstAV1, dstHEVC, originalFilename, progressFile string, wAV1, hAV1, wHEVC, hHEVC int) (*exec.Cmd, error) {
	cmd := exec.Command("ffmpeg", "-y", "-i", originalFilename, "-c:v", "libaom-av1", "-vf", fmt.Sprintf("scale=%dx%d,fps=10", wAV1, hAV1), "-c:a", "aac", "-ac", "1", "-b:a", "24k", "-crf", "42", "-b:v", "0", "-progress", progressFile, dstAV1, "&&",
		"ffmpeg", "-y", "-i", originalFilename, "-c:v", "libx265", "-vf", fmt.Sprintf("scale=%dx%d,fps=10", wHEVC, hHEVC), "-c:a", "aac", "-ac", "1", "-b:a", "24k", "-crf", "42", "-b:v", "0", "-progress", progressFile, dstHEVC,
	)
	e := cmd.Start()
	if e != nil {
		return nil, e
	}
	return cmd, nil
}

// ffmpeg -i a.mp4 -c:v libx265 -vf scale=640x360,fps=10 -c:a aac -ac 1 -b:a 24k  -crf 42 -b:v 0 out.hevc.mp4
func compressToHEVC(dst, filename, progressFile string, w, h int) (*exec.Cmd, error) {
	cmd := exec.Command("ffmpeg", "-i", filename, "-c:v", "libx265", "-vf", fmt.Sprintf("scale=%dx%d,fps=10", w, h), "-c:a", "aac", "-ac", "1", "-b:a", "24k", "-crf", "42", "-b:v", "0", "-progress", progressFile, dst)
	e := cmd.Start()
	if e != nil {
		return nil, e
	}
	return cmd, nil
}

// ffmpeg -i a.mp4 -c:v libaom-av1 -vf scale=256x144,fps=10 -c:a aac -ac 1 -b:a 24k  -crf 42 -b:v 0 out.av1.mp4
func compressToAV1(dst, filename, progressFile string, w, h int) (*exec.Cmd, error) {
	cmd := exec.Command("ffmpeg", "-i", filename, "-c:v", "libaom-av1", "-vf", fmt.Sprintf("scale=%dx%d,fps=10", w, h), "-c:a", "aac", "-ac", "1", "-b:a", "24k", "-crf", "42", "-b:v", "0", "-progress", progressFile, dst)
	e := cmd.Start()
	if e != nil {
		return nil, e
	}
	return cmd, nil
}

func ProbeVideoAuto(filename string) (*MediaInfo, error) {
	oneFrame := filename + ".1f" + filepath.Ext(filename)
	e := cmdToolkit.RunAttach("ffmpeg", "-i", filename, "-frames:v", "1", oneFrame)
	if e != nil {
		log.Println(e)
		return nil, e
	}
	defer os.Remove(oneFrame)

	cover, e := ProbeMedia(oneFrame)
	if e != nil {
		log.Println(e)
		return nil, e
	}

	info, e := ProbeMedia(filename)
	if e != nil {
		log.Println(e)
		return nil, e
	}
	info.Width = cover.Width
	info.Height = cover.Height

	return info, nil
}

/*
Stream #0:1[0x2](und): Video: h264 (High) (avc1 / 0x31637661), yuvj420p(pc, bt709/bt709/iec61966-2-1, progressive), 828x1792, 16366 kb/s, 58.90 fps, 60 tbr, 600 tbn (default)
Metadata:

	creation_time   : 2023-05-14T03:39:10.000000Z
	handler_name    : Core Media Video
	vendor_id       : [0][0][0][0]

Side data:

	displaymatrix: rotation of 90.00 degrees
*/
func ProbeMedia(filename string) (*MediaInfo, error) {
	output, e := cmdToolkit.Run("ffprobe", filename)
	if e != nil {
		log.Println(e)
		return nil, e
	}
	ss := strings.Split(output, "\n")
	if len(ss) < 2 {
		return nil, errors.New("invalid output:" + output)
	}

	var width, height int
	var dur int
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if width == 0 && strings.HasPrefix(s, "Stream") && strings.Contains(s, "Video:") {
			s = strToolkit.SubAfter(s, "Video:", "")
			for _, item := range strings.Split(s, ", ") {
				item = strings.TrimSpace(item)
				if !strings.Contains(item, "x") {
					continue
				}
				item = strToolkit.SubBefore(item, " ", item)
				vs := strings.Split(item, "x")
				if len(vs) != 2 {
					continue
				}
				width, e = strconv.Atoi(vs[0])
				if e != nil {
					continue
				}
				height, e = strconv.Atoi(vs[1])
				if e != nil {
					continue
				}
			}
			continue
		}

		if dur == 0 && strings.HasPrefix(s, "Duration:") {
			s = strToolkit.SubAfter(s, "Duration:", s)
			s = strToolkit.SubBefore(s, ",", s)
			s = strings.TrimSpace(s)
			s = strToolkit.SubBeforeLast(s, ".", s)
			if s != "N/A" {
				dur, e = tools.ParseDurationSeconds(s)
				if e != nil {
					return nil, fmt.Errorf("parse duration '%s' failed:%w", s, e)
				}
			}

			continue
		}
	}

	return &MediaInfo{
		Width:           width,
		Height:          height,
		DurationSeconds: dur,
	}, nil
}

// ProbeAudio returns audio ext,duration
// parsing output example :
//
// "Input #0, wav, from 'b.wav':
// Duration: 00:01:00.96, bitrate: 1536 kb/s
// Stream #0:0: Audio: pcm_s16le ([1][0][0][0] / 0x0001), 48000 Hz, 2 channels, s16, 1536 kb/s"
func ProbeAudio(filename string) (string, int, error) {
	cmd := exec.Command("ffprobe", filename)
	bufErr := new(bytes.Buffer)
	bufOut := new(bytes.Buffer)
	cmd.Stderr = bufErr
	cmd.Stdout = bufOut
	e := cmd.Run()
	if e != nil {
		if strings.Contains(e.Error(), "exit status") {
			return "", 0, errors.New(trimPrefix(bufErr.String()))
		}
		return "", 0, e
	}
	output := trimPrefix(bufErr.String())
	ss := strings.Split(output, "\n")
	if len(ss) < 2 {
		return "", 0, errors.New("invalid output:" + output)
	}
	ext := ""
	var seconds int
	for _, s := range ss {
		if ext == "" && strings.HasPrefix(s, "Stream") {
			seps := strings.Split(strToolkit.SubAfter(s, "Audio: ", s), ", ")
			if len(seps) < 2 {
				return "", 0, errors.New("invalid ext line:" + output)
			}
			ext = "." + strToolkit.SubBefore(seps[0], " ", seps[0])
			continue
		}
		if seconds == 0 && strings.HasPrefix(s, "Duration") {
			str := strToolkit.SubBefore(s, ", ", s)
			str = strToolkit.SubAfter(str, ": ", str)
			str = strToolkit.SubBeforeLast(str, ".", str)
			if str == "N/A" {
				return "", 0, errors.New("input " + ext + " is not audio format")
			}
			seconds, e = tools.ParseDurationSeconds(str)
			continue
		}
	}
	return ext, seconds, nil
}

func trimPrefix(s string) string {
	b := new(strings.Builder)
	ss := strings.Split(s, "\n")
	hadLibs := false
	for _, line := range ss {
		line = strToolkit.TrimStarts(line, " ")
		if strings.HasPrefix(line, "lib") {
			hadLibs = true
			continue
		}
		if !hadLibs {
			continue
		}
		b.WriteString(line + "\n")
	}
	return strToolkit.TrimEnds(b.String(), "\n")
}
