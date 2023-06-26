package core

import (
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenZack/tools/strToolkit"
	"github.com/StevenZack/transcoder/internal/ffmpegx"
	"github.com/StevenZack/transcoder/internal/tools"
)

type (
	Task struct {
		Id     string `json:"id"`
		User   string `json:"user"`
		Origin string `json:"origin"`
		Ext    string `json:"ext"`
		Mime   string `json:"mime"`

		MediaInfo    *ffmpegx.MediaInfo    `json:"media_info"`
		ProgressInfo *ffmpegx.ProgressInfo `json:"progress_info"`
		Cmd          *exec.Cmd             `json:"-"`
		ProgressFile string                `json:"-"`
		IsEnded      bool                  `json:"isEnded"`

		PublicUrl   string   `json:"public_url"`   //
		OutputFiles []string `json:"output_files"` // output urls
		CreateAt    string   `json:"create_at"`
	}
)

var (
	TaskMap = new(tools.Map[string, Task])
	AppDir  = filepath.Join(os.TempDir(), PACKAGE_NAME)
)

func init() {
	e := os.MkdirAll(AppDir, 0755)
	if e != nil {
		log.Println(e)
		return
	}

}

func CreateTask(fh *multipart.FileHeader, user string) (*Task, error) {
	v := &Task{
		Id:       tools.GenerateID(),
		Origin:   fh.Filename,
		Ext:      filepath.Ext(fh.Filename),
		User:     user,
		CreateAt: time.Now().Format(time.RFC3339),
	}
	v.Mime = mime.TypeByExtension(v.Ext)

	v.Origin = filepath.Join(AppDir, v.Id+v.Ext)
	e := tools.ReadFileHeader(v.Origin, fh)
	if e != nil {
		log.Println(e)
		return nil, e
	}

	switch strToolkit.SubBefore(v.Mime, "/", v.Mime) {
	case "image":
		// media_info
		v.MediaInfo, e = ffmpegx.ProbeMedia(v.Origin)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		filename := fmt.Sprintf("%s@%dx%d", v.Id, v.MediaInfo.Width, v.MediaInfo.Height)
		// avif
		avif := filepath.Join(AppDir, filename+".avif")
		e = ffmpegx.CompressImage(avif, v.Origin)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		v.OutputFiles = append(v.OutputFiles, filename+".avif")

		// webp
		webp := filepath.Join(AppDir, filename+".webp")
		e = ffmpegx.CompressImage(webp, v.Origin)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		v.OutputFiles = append(v.OutputFiles, filename+".webp")

		v.PublicUrl = filename + ".avif"

		// delete v.Origin
		// e = os.Remove(v.Origin)
		// if e != nil {
		// 	log.Println(e)
		// 	return nil, e
		// }
		v.IsEnded = true
	case "video":
		// media_info
		v.MediaInfo, e = ffmpegx.ProbeVideoAuto(v.Origin)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		w, h := ffmpegx.FitConstraint(ffmpegx.MAX_AV1_CONSTRAINT, ffmpegx.MAX_HEVC_CONSTRAINT, v.MediaInfo.Width, v.MediaInfo.Height)
		filename := fmt.Sprintf("%s@%dx%d", v.Id, w, h)
		v.ProgressFile = filepath.Join(AppDir, filename+".progress.txt")
		// cover
		coverAvif := filepath.Join(AppDir, filename+".cover.avif")
		e = ffmpegx.CreateCoverOfVideo(coverAvif, v.Origin, w, h)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		v.OutputFiles = append(v.OutputFiles, filename+".cover.avif")

		// video
		wHEVC, hHEVC := ffmpegx.FitConstraint(ffmpegx.MAX_HEVC_CONSTRAINT, ffmpegx.MAX_HEVC_CONSTRAINT, v.MediaInfo.Width, v.MediaInfo.Height)
		av1 := filepath.Join(AppDir, filename+".av1.mp4")
		hevc := filepath.Join(AppDir, filename+".hevc.mp4")
		v.Cmd, e = ffmpegx.CompressToAV1_HEVC(av1, hevc, v.Origin, v.ProgressFile, w, h, wHEVC, hHEVC)
		if e != nil {
			log.Println(e)
			return nil, e
		}

		v.OutputFiles = append(v.OutputFiles, av1, hevc)

	default:
		return nil, errors.New("Unsupported file type :" + v.Mime)
	}

	return v, nil
}
func (t *Task) currentProgress() (*ffmpegx.ProgressInfo, error) {
	if t.ProgressFile == "" {
		return &ffmpegx.ProgressInfo{
			Progress: ffmpegx.PROGRESS_END,
		}, nil
	}
	return ffmpegx.TailProgressFile(t.ProgressFile)
}

func (t *Task) LoadProgress() error {
	if t.IsEnded {
		return nil
	}
	if strings.HasPrefix(t.Mime, "video/") {
		var e error
		t.ProgressInfo, e = t.currentProgress()
		if e != nil {
			log.Println(e)
			return e
		}
	}
	return nil
}

func (t *Task) CheckIsEnded() bool {
	if t.IsEnded {
		return true
	}
	if strings.HasPrefix(t.Mime, "video/") {
		info, e := t.currentProgress()
		if e != nil {
			log.Println(e)
			return true
		}
		return info.Progress == ffmpegx.PROGRESS_END
	}
	return true
}

func (t *Task) Clean() {
	os.Remove(t.Origin)
	if t.ProgressFile != "" {
		os.Remove(t.ProgressFile)
	}
	for _, output := range t.OutputFiles {
		e := os.Remove(output)
		if e != nil {
			log.Println(e)
		}
	}
}
