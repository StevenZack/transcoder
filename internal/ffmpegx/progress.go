package ffmpegx

import (
	"strconv"
	"strings"

	"github.com/StevenZack/tools/cmdToolkit"
	"github.com/StevenZack/transcoder/internal/tools"
)

/*
*
frame=7233
fps=48.76
stream_0_0_q=0.0
bitrate=N/A
total_size=762
out_time_us=0
out_time_ms=0
out_time=00:00:00.000000
dup_frames=0
drop_frames=0
speed=   0x
progress=end
*/
type ProgressInfo struct {
	Frame          int    `json:"frame"`
	OutTime        string `json:"out_time"`
	OutTimeSeconds int    `json:"out_time_seconds"`
	Speed          string `json:"speed"`
	Progress       string `json:"progress"` // continue|end
}

const (
	PROGRESS_END      = "end"
	PROGRESS_CONTINUE = "continue"
)

func TailProgressFile(filename string) (*ProgressInfo, error) {
	str, e := cmdToolkit.Run("tail", "-n", "12", filename)
	if e != nil {
		return nil, e
	}
	out := new(ProgressInfo)
	for _, s := range strings.Split(str, "\n") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		kv := strings.Split(s, "=")
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := kv[1]
		switch key {
		case "frame":
			i, e := strconv.Atoi(value)
			if e != nil {
				return nil, e
			}
			out.Frame = i
		case "out_time":
			out.OutTime = value
			out.OutTimeSeconds, e = tools.ParseDurationSeconds(value)
			if e != nil {
				return nil, e
			}
		case "speed":
			out.Speed = value
		case "progress":
			out.Progress = value
		}
	}
	return out, nil
}
