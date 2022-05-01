package filters

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/StollD/videoproc/common"
	"github.com/StollD/videoproc/mkv"
	"github.com/codeskyblue/go-sh"
	"github.com/fatih/color"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

func ExtractStream(stream mkv.Stream, out string) error {
	fmt.Printf("  Extracting stream %s\t", stream.ID())

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		color.Green("✓")
		return nil
	}

	sel := fmt.Sprintf("0:%d", stream.Index())

	temp := common.Temp(out)
	_, err := sh.Command(common.FFMPEG, "-i", stream.Path(), "-map", sel, "-codec", "copy", "-y", temp).CombinedOutput()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, out)

	color.Green("✓")
	return nil
}

type ExtractedVideoStream struct {
	mkv.VideoStream

	dir  string
	base mkv.VideoStream
}

// Extract the video stream into a single mkv file
func ExtractVideo(base mkv.VideoStream, dir string) mkv.VideoStream {
	return &ExtractedVideoStream{base: base, dir: dir}
}

func (video *ExtractedVideoStream) Probe() map[string]interface{} {
	return video.base.Probe()
}

func (video *ExtractedVideoStream) MediaInfo() map[string]interface{} {
	return video.base.MediaInfo()
}

func (video *ExtractedVideoStream) ID() string {
	return video.base.ID()
}

func (video *ExtractedVideoStream) Type() string {
	return video.base.Type()
}

func (video *ExtractedVideoStream) Index() int {
	return 0
}

func (video *ExtractedVideoStream) Language() string {
	return video.base.Language()
}

func (video *ExtractedVideoStream) Offset() float64 {
	return video.base.Offset()
}

func (video *ExtractedVideoStream) Path() string {
	return filepath.Join(video.dir, fmt.Sprintf("%s.mkv", video.ID()))
}

func (video *ExtractedVideoStream) Prepare() error {
	err := video.base.Prepare()
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = ExtractStream(video.base, video.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *ExtractedVideoStream) Process() error {
	return tracerr.Wrap(video.base.Process())
}

func (video *ExtractedVideoStream) Cleanup() error {
	return nil
}

func (video *ExtractedVideoStream) Aspect() string {
	return video.base.Aspect()
}

func (video *ExtractedVideoStream) Framerate() mkv.Framerate {
	return video.base.Framerate()
}

func (video *ExtractedVideoStream) Speedup() float64 {
	return video.base.Speedup()
}

type ExtractedAudioStream struct {
	mkv.AudioStream

	dir  string
	base mkv.AudioStream
}

// Extract the audio stream into a single mkv file
func ExtractAudio(base mkv.AudioStream, dir string) mkv.AudioStream {
	return &ExtractedAudioStream{base: base, dir: dir}
}

func (audio *ExtractedAudioStream) Probe() map[string]interface{} {
	return audio.base.Probe()
}

func (audio *ExtractedAudioStream) MediaInfo() map[string]interface{} {
	return audio.base.MediaInfo()
}

func (audio *ExtractedAudioStream) ID() string {
	return audio.base.ID()
}

func (audio *ExtractedAudioStream) Type() string {
	return audio.base.Type()
}

func (audio *ExtractedAudioStream) Index() int {
	return 0
}

func (audio *ExtractedAudioStream) Language() string {
	return audio.base.Language()
}

func (audio *ExtractedAudioStream) Offset() float64 {
	return audio.base.Offset()
}

func (audio *ExtractedAudioStream) Path() string {
	return filepath.Join(audio.dir, fmt.Sprintf("%s.mkv", audio.ID()))
}

func (audio *ExtractedAudioStream) Prepare() error {
	err := audio.base.Prepare()
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = ExtractStream(audio.base, audio.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *ExtractedAudioStream) Process() error {
	return tracerr.Wrap(audio.base.Process())
}

func (audio *ExtractedAudioStream) Cleanup() error {
	return nil
}

func (audio *ExtractedAudioStream) Samplerate() int {
	return audio.base.Samplerate()
}

type ExtractedSubtitleStream struct {
	mkv.SubtitleStream

	dir  string
	base mkv.SubtitleStream
}

// Extract the subtitle stream into a single mkv file
func ExtractSubtitle(base mkv.SubtitleStream, dir string) mkv.SubtitleStream {
	return &ExtractedSubtitleStream{base: base, dir: dir}
}

func (sub *ExtractedSubtitleStream) Probe() map[string]interface{} {
	return sub.base.Probe()
}

func (sub *ExtractedSubtitleStream) MediaInfo() map[string]interface{} {
	return sub.base.MediaInfo()
}

func (sub *ExtractedSubtitleStream) ID() string {
	return sub.base.ID()
}

func (sub *ExtractedSubtitleStream) Type() string {
	return sub.base.Type()
}

func (sub *ExtractedSubtitleStream) Index() int {
	return 0
}

func (sub *ExtractedSubtitleStream) Language() string {
	return sub.base.Language()
}

func (sub *ExtractedSubtitleStream) Offset() float64 {
	return sub.base.Offset()
}

func (sub *ExtractedSubtitleStream) Path() string {
	return filepath.Join(sub.dir, fmt.Sprintf("%s.mkv", sub.ID()))
}

func (sub *ExtractedSubtitleStream) Prepare() error {
	err := sub.base.Prepare()
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = ExtractStream(sub.base, sub.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (sub *ExtractedSubtitleStream) Process() error {
	return tracerr.Wrap(sub.base.Process())
}

func (sub *ExtractedSubtitleStream) Cleanup() error {
	return nil
}
