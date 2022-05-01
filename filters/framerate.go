package filters

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/StollD/videoproc/common"
	"github.com/StollD/videoproc/mkv"
	"github.com/codeskyblue/go-sh"
	"github.com/fatih/color"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

type FramerateVideoStream struct {
	mkv.VideoStream

	dir       string
	framerate mkv.Framerate
	base      mkv.VideoStream
}

// Change the framerate of the video stream
func ChangeVideoFramerate(base mkv.VideoStream, dir string, framerate string) mkv.VideoStream {
	video := &FramerateVideoStream{base: base, dir: dir}
	video.framerate.Parse(framerate)
	return video
}

func (video *FramerateVideoStream) Probe() map[string]interface{} {
	return video.base.Probe()
}

func (video *FramerateVideoStream) MediaInfo() map[string]interface{} {
	return video.base.MediaInfo()
}

func (video *FramerateVideoStream) ID() string {
	return video.base.ID()
}

func (video *FramerateVideoStream) Type() string {
	return video.base.Type()
}

func (video *FramerateVideoStream) Index() int {
	return video.base.Index()
}

func (video *FramerateVideoStream) Language() string {
	return video.base.Language()
}

func (video *FramerateVideoStream) Offset() float64 {
	return video.base.Offset() / video.RelativeSpeedup()
}

func (video *FramerateVideoStream) Path() string {
	return filepath.Join(video.dir, fmt.Sprintf("%s.fps.mkv", video.ID()))
}

func (video *FramerateVideoStream) Prepare() error {
	return tracerr.Wrap(video.base.Prepare())
}

func (video *FramerateVideoStream) Process() error {
	// The file already exists
	if _, err := os.Stat(video.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process earlier stages
	err := video.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Changing FPS of stream %s to %s\t", video.ID(), video.Framerate().String())

	temp := common.Temp(video.Path())

	args := make([]interface{}, 0)
	args = append(args, "-o", temp)
	args = append(args, "-default-duration", fmt.Sprintf("%d:%sp", video.Index(), video.Framerate().String()))
	args = append(args, "--fix-bitstream-timing-information", fmt.Sprintf("%d:true", video.Index()))
	args = append(args, "--chapter-sync", fmt.Sprintf("0,1/%f", video.RelativeSpeedup()))
	args = append(args, video.base.Path())

	_, err = sh.Command(common.MKVMERGE, args...).CombinedOutput()
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

func (video *FramerateVideoStream) Cleanup() error {
	err := os.Remove(video.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *FramerateVideoStream) Aspect() string {
	return video.base.Aspect()
}

func (video *FramerateVideoStream) Framerate() mkv.Framerate {
	return video.framerate
}

func (video *FramerateVideoStream) Speedup() float64 {
	return video.base.Speedup() * video.RelativeSpeedup()
}

func (video *FramerateVideoStream) RelativeSpeedup() float64 {
	return video.Framerate().Value() / video.base.Framerate().Value()
}

type SyncedAudioStream struct {
	mkv.AudioStream

	dir   string
	video mkv.VideoStream
	base  mkv.AudioStream

	options map[string]string
}

func SyncAudio(base mkv.AudioStream, video mkv.VideoStream, dir string, options map[string]string) mkv.AudioStream {
	return &SyncedAudioStream{base: base, video: video, dir: dir, options: options}
}

func (audio *SyncedAudioStream) Probe() map[string]interface{} {
	return audio.base.Probe()
}

func (audio *SyncedAudioStream) MediaInfo() map[string]interface{} {
	return audio.base.MediaInfo()
}

func (audio *SyncedAudioStream) ID() string {
	return audio.base.ID()
}

func (audio *SyncedAudioStream) Type() string {
	return audio.base.Type()
}

func (audio *SyncedAudioStream) Index() int {
	return 0
}

func (audio *SyncedAudioStream) Language() string {
	return audio.base.Language()
}

func (audio *SyncedAudioStream) Offset() float64 {
	return audio.base.Offset()
}

func (audio *SyncedAudioStream) Path() string {
	return filepath.Join(audio.dir, fmt.Sprintf("%s.fps.mkv", audio.ID()))
}

func (audio *SyncedAudioStream) Prepare() error {
	return tracerr.Wrap(audio.base.Prepare())
}

func (audio *SyncedAudioStream) Process() error {
	// The file already exists
	if _, err := os.Stat(audio.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process earlier stages
	err := audio.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Syncing audio stream %s to video\t", audio.ID())

	temp := common.Temp(audio.Path())

	args := make([]interface{}, 0)
	args = append(args, "-i", audio.base.Path())
	args = append(args, "-map", fmt.Sprintf("0:%d", audio.base.Index()))

	keep_pitch, exists := audio.options["keep_pitch"]
	if !exists {
		keep_pitch = "false"
	}

	kp, err := strconv.ParseBool(keep_pitch)
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	filter := fmt.Sprintf("asetrate=%d*%f,aresample", audio.Samplerate(), audio.video.Speedup())
	if kp {
		filter = fmt.Sprintf("atempo=%f", audio.video.Speedup())
	}

	args = append(args, "-af", filter)
	args = append(args, "-ar", fmt.Sprintf("%d", audio.Samplerate()))

	resampler, exists := audio.options["resampler"]
	if exists {
		args = append(args, "-resampler", resampler)
	}

	args = append(args, "-codec", "pcm_f32le")
	args = append(args, "-y", temp)

	_, err = sh.Command(common.FFMPEG, args...).CombinedOutput()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, audio.Path())
	color.Green("✓")

	err = audio.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *SyncedAudioStream) Cleanup() error {
	err := os.Remove(audio.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *SyncedAudioStream) Samplerate() int {
	return audio.base.Samplerate()
}

type SyncedSubtitleStream struct {
	mkv.SubtitleStream

	dir   string
	video mkv.VideoStream
	base  mkv.SubtitleStream
}

func SyncSubtitle(base mkv.SubtitleStream, video mkv.VideoStream, dir string) mkv.SubtitleStream {
	return &SyncedSubtitleStream{base: base, video: video, dir: dir}
}

func (sub *SyncedSubtitleStream) Probe() map[string]interface{} {
	return sub.base.Probe()
}

func (sub *SyncedSubtitleStream) MediaInfo() map[string]interface{} {
	return sub.base.MediaInfo()
}

func (sub *SyncedSubtitleStream) ID() string {
	return sub.base.ID()
}

func (sub *SyncedSubtitleStream) Type() string {
	return sub.base.Type()
}

func (sub *SyncedSubtitleStream) Index() int {
	return 0
}

func (sub *SyncedSubtitleStream) Language() string {
	return sub.base.Language()
}

func (sub *SyncedSubtitleStream) Offset() float64 {
	return sub.base.Offset()
}

func (sub *SyncedSubtitleStream) Path() string {
	return filepath.Join(sub.dir, fmt.Sprintf("%s.fps.mkv", sub.ID()))
}

func (sub *SyncedSubtitleStream) Prepare() error {
	return sub.base.Prepare()
}

func (sub *SyncedSubtitleStream) Process() error {
	// The file already exists
	if _, err := os.Stat(sub.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process earlier stages
	err := sub.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Syncing subtitle stream %s to video\t", sub.ID())

	temp := common.Temp(sub.Path())

	args := make([]interface{}, 0)
	args = append(args, "-i", sub.base.Path())
	args = append(args, "-map", fmt.Sprintf("0:%d", sub.base.Index()))

	filter := fmt.Sprintf("setts=TS/%f", sub.video.Speedup())
	args = append(args, "-bsf", filter)

	args = append(args, "-codec", "copy")
	args = append(args, "-y", temp)

	_, err = sh.Command(common.FFMPEG, args...).CombinedOutput()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, sub.Path())
	color.Green("✓")

	// Cleanup
	err = sub.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (sub *SyncedSubtitleStream) Cleanup() error {
	err := os.Remove(sub.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}
