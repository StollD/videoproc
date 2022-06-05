package filters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StollD/videoproc/common"
	"github.com/StollD/videoproc/mkv"
	"github.com/codeskyblue/go-sh"
	"github.com/fatih/color"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

type VapoursynthVideoStream struct {
	mkv.VideoStream

	dir      string
	template string
	script   string
	base     mkv.VideoStream
}

func VapoursynthFilter(base mkv.VideoStream, dir string, filter string) mkv.VideoStream {
	return &VapoursynthVideoStream{base: base, dir: dir, template: filter}
}

func (video *VapoursynthVideoStream) Probe() map[string]interface{} {
	return video.base.Probe()
}

func (video *VapoursynthVideoStream) MediaInfo() map[string]interface{} {
	return video.base.MediaInfo()
}

func (video *VapoursynthVideoStream) ID() string {
	return video.base.ID()
}

func (video *VapoursynthVideoStream) Type() string {
	return video.base.Type()
}

func (video *VapoursynthVideoStream) Index() int {
	return 0
}

func (video *VapoursynthVideoStream) Language() string {
	return video.base.Language()
}

func (video *VapoursynthVideoStream) Offset() float64 {
	return video.base.Offset()
}

func (video *VapoursynthVideoStream) Path() string {
	return filepath.Join(video.dir, fmt.Sprintf("%s.vpy.mkv", video.ID()))
}

func (video *VapoursynthVideoStream) Prepare() error {
	err := video.base.Prepare()
	if err != nil {
		return tracerr.Wrap(err)
	}

	video.script = filepath.Join(video.dir, fmt.Sprintf("%s.vpy", video.ID()))

	// The file already exists
	if _, err := os.Stat(video.script); !os.IsNotExist(err) {
		return nil
	}

	filter, err := os.ReadFile(video.template)
	if err != nil {
		return tracerr.Wrap(err)
	}

	new := string(filter)
	new = strings.ReplaceAll(new, "$(video)$", video.base.Path())
	new = strings.ReplaceAll(new, "$(filter)$", video.template)

	temp := common.Temp(video.script)
	err = os.WriteFile(temp, []byte(new), 0644)
	if err != nil {
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, video.script)
	return nil
}

func (video *VapoursynthVideoStream) Process() error {
	if _, err := os.Stat(video.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process earlier stages
	err := video.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	temp := common.Temp(video.Path())

	fmt.Printf("  Filtering stream %s using %s\t", video.ID(), filepath.Base(video.template))

	vspipe := sh.Command(common.VSPIPE, video.script, "-", "-c", "y4m")
	vspipe.PipeStdErrors = true

	args := make([]interface{}, 0)
	args = append(args, "-r", video.Framerate().String())
	args = append(args, "-i", "pipe:")
	args = append(args, "-i", video.base.Path())
	args = append(args, "-codec", "ffv1")
	args = append(args, "-map", "0")
	args = append(args, "-map_metadata", "1")
	args = append(args, "-map_chapters", "1")
	args = append(args, "-aspect", video.Aspect())
	args = append(args, "-y", temp)

	cmd := vspipe.Command(common.FFMPEG, args...)
	cmd.Stderr = common.DevNull()

	err = cmd.Run()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, video.Path())
	color.Green("✓")

	// Cleanup
	err = video.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *VapoursynthVideoStream) Cleanup() error {
	err := os.Remove(video.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *VapoursynthVideoStream) Aspect() string {
	return video.base.Aspect()
}

func (video *VapoursynthVideoStream) Framerate() mkv.Framerate {
	return video.base.Framerate()
}

func (video *VapoursynthVideoStream) Speedup() float64 {
	return video.base.Speedup()
}

func (video *VapoursynthVideoStream) Frames() int {
	shell := sh.Command(common.FFPROBE, "-show_streams", "-f", "vapoursynth", "-of", "json", video.script)
	shell.Stderr = common.DevNull()

	var probe map[string]interface{}
	err := shell.UnmarshalJSON(&probe)
	if err != nil {
		return -1
	}

	streams := probe["streams"].([]interface{})
	stream := streams[0].(map[string]interface{})

	// When loading a vapoursynth script through ffmpeg,
	// the timebase is the inverted FPS, and the amount of
	// timestamps is equal to the amount of frames.
	return int(stream["duration_ts"].(float64))
}
