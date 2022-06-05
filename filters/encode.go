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

type EncodedVideoStream struct {
	mkv.VideoStream

	dir   string
	base  mkv.VideoStream
	codec map[string]string
}

func EncodeVideo(base mkv.VideoStream, dir string, codec map[string]string) mkv.VideoStream {
	return &EncodedVideoStream{base: base, dir: dir, codec: codec}
}

func (video *EncodedVideoStream) Probe() map[string]interface{} {
	return video.base.Probe()
}

func (video *EncodedVideoStream) MediaInfo() map[string]interface{} {
	return video.base.MediaInfo()
}

func (video *EncodedVideoStream) ID() string {
	return video.base.ID()
}

func (video *EncodedVideoStream) Type() string {
	return video.base.Type()
}

func (video *EncodedVideoStream) Index() int {
	return 0
}

func (video *EncodedVideoStream) Language() string {
	return video.base.Language()
}

func (video *EncodedVideoStream) Offset() float64 {
	return video.base.Offset()
}

func (video *EncodedVideoStream) Path() string {
	return filepath.Join(video.dir, fmt.Sprintf("%s.enc.mkv", video.ID()))
}

func (video *EncodedVideoStream) Prepare() error {
	return tracerr.Wrap(video.base.Prepare())
}

func (video *EncodedVideoStream) Process() error {
	// The file already exists
	if _, err := os.Stat(video.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process previous stages
	err := video.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Encoding stream %s using codec %s\t", video.ID(), video.codec["codec"])

	temp := common.Temp(video.Path())

	args := make([]interface{}, 0)
	args = append(args, "-r", video.Framerate().String())
	args = append(args, "-i", video.base.Path())
	args = append(args, "-map", fmt.Sprintf("0:%d", video.base.Index()))
	args = append(args, "-aspect", video.Aspect())

	for key, val := range video.codec {
		if key == "bitrate" {
			key = "b:v"
		}

		args = append(args, fmt.Sprintf("-%s", key), val)
	}

	// Is this a two-pass encode?
	_, twopass := video.codec["bitrate"]

	if !twopass {
		args = append(args, "-y", temp)

		_, err := sh.Command(common.FFMPEG, args...).CombinedOutput()
		if err != nil {
			color.Red("✗")
			return tracerr.Wrap(err)
		}
	} else {
		p1 := append(args, "-pass", "1", "-f", "null", "-")

		_, err := sh.Command(common.FFMPEG, p1...).CombinedOutput()
		if err != nil {
			color.Red("✗")
			return tracerr.Wrap(err)
		}

		p2 := append(args, "-pass", "2", "-y", temp)

		_, err = sh.Command(common.FFMPEG, p2...).CombinedOutput()
		if err != nil {
			color.Red("✗")
			return tracerr.Wrap(err)
		}
	}

	atomic.ReplaceFile(temp, video.Path())
	color.Green("✓")

	// Cleanuo
	err = video.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *EncodedVideoStream) Cleanup() error {
	err := os.Remove(video.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (video *EncodedVideoStream) Aspect() string {
	return video.base.Aspect()
}

func (video *EncodedVideoStream) Framerate() mkv.Framerate {
	return video.base.Framerate()
}

func (video *EncodedVideoStream) Speedup() float64 {
	return video.base.Speedup()
}

type EncodedAudioStream struct {
	mkv.AudioStream

	dir   string
	base  mkv.AudioStream
	codec map[string]string
}

// Extract the audio stream into a single mkv file
func EncodeAudio(base mkv.AudioStream, dir string, codec map[string]string) mkv.AudioStream {
	return &EncodedAudioStream{base: base, dir: dir, codec: codec}
}

func (audio *EncodedAudioStream) Probe() map[string]interface{} {
	return audio.base.Probe()
}

func (audio *EncodedAudioStream) MediaInfo() map[string]interface{} {
	return audio.base.MediaInfo()
}

func (audio *EncodedAudioStream) ID() string {
	return audio.base.ID()
}

func (audio *EncodedAudioStream) Type() string {
	return audio.base.Type()
}

func (audio *EncodedAudioStream) Index() int {
	return 0
}

func (audio *EncodedAudioStream) Language() string {
	return audio.base.Language()
}

func (audio *EncodedAudioStream) Offset() float64 {
	return audio.base.Offset()
}

func (audio *EncodedAudioStream) Path() string {
	return filepath.Join(audio.dir, fmt.Sprintf("%s.enc.mkv", audio.ID()))
}

func (audio *EncodedAudioStream) Prepare() error {
	return tracerr.Wrap(audio.base.Prepare())
}

func (audio *EncodedAudioStream) Process() error {
	// The file already exists
	if _, err := os.Stat(audio.Path()); !os.IsNotExist(err) {
		return nil
	}

	// Process previous stages
	err := audio.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Encoding stream %s using codec %s\t", audio.ID(), audio.codec["codec"])

	temp := common.Temp(audio.Path())

	args := make([]interface{}, 0)
	args = append(args, "-i", audio.base.Path())
	args = append(args, "-map", fmt.Sprintf("0:%d", audio.base.Index()))

	// Fix for converting ac3 to opus
	codec, exists := audio.codec["codec"]
	if codec == "libopus" && exists {
		af, exists := audio.codec["af"]
		of := "channelmap=channel_layout=5.1"

		probe := audio.Probe()
		layout := probe["channel_layout"].(string)

		if layout == "5.1(side)" {
			if exists {
				af = af + of
			} else {
				af = of
			}

			audio.codec["af"] = af
		}
	}

	for key, val := range audio.codec {
		if key == "bitrate" {
			key = "b:a"
		}

		// Apply values from Mediainfos extra data to the codec
		info := audio.MediaInfo()
		v, exists := info["extra"]

		if exists {
			extra := v.(map[string]interface{})

			for k2, v2 := range extra {
				val = strings.ReplaceAll(val, fmt.Sprintf("$(%s)$", k2), v2.(string))
			}
		}

		match, err := filepath.Match("$(*)$", val)
		if err == nil && !match {
			args = append(args, fmt.Sprintf("-%s", key), val)
		}
	}

	args = append(args, "-y", temp)

	_, err = sh.Command(common.FFMPEG, args...).CombinedOutput()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, audio.Path())
	color.Green("✓")

	// Cleanup
	err = audio.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *EncodedAudioStream) Cleanup() error {
	err := os.Remove(audio.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *EncodedAudioStream) Samplerate() int {
	return audio.base.Samplerate()
}
