package main

import (
	"os/exec"

	"github.com/StevenZack/transcoder/internal/ffmpegx"
	"github.com/StevenZack/transcoder/internal/tools"
)

type (
	Task struct {
		Id           string             `json:"id"`
		MediaInfo    *ffmpegx.MediaInfo `json:"media_info"`
		Cmd          *exec.Cmd          `json:"-"`
		ProgressFile string             `json:"progress_file"`
	}
)

var taskMap *tools.Map[string, *Task]

func (t *Task) CurrentProgress() (*ffmpegx.ProgressInfo, error) {
	return ffmpegx.TailProgressFile(t.ProgressFile)
}
