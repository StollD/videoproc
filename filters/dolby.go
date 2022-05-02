package filters

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/StollD/videoproc/common"
	"github.com/StollD/videoproc/mkv"
	"github.com/codeskyblue/go-sh"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

type DolbyAudioStream struct {
	mkv.AudioStream

	dir  string
	base mkv.AudioStream
}

func NormalizeDolby(base mkv.AudioStream, dir string) mkv.AudioStream {
	probe := base.Probe()

	val, exists := probe["codec_name"]
	if !exists {
		return base
	}

	if val.(string) != "ac3" {
		return base
	}

	return &DolbyAudioStream{base: base, dir: dir}
}

func (audio *DolbyAudioStream) Probe() map[string]interface{} {
	return audio.base.Probe()
}

func (audio *DolbyAudioStream) MediaInfo() map[string]interface{} {
	return audio.base.MediaInfo()
}

func (audio *DolbyAudioStream) ID() string {
	return audio.base.ID()
}

func (audio *DolbyAudioStream) Type() string {
	return audio.base.Type()
}

func (audio *DolbyAudioStream) Index() int {
	return 0
}

func (audio *DolbyAudioStream) Language() string {
	return audio.base.Language()
}

func (audio *DolbyAudioStream) Offset() float64 {
	return audio.base.Offset()
}

func (audio *DolbyAudioStream) Path() string {
	return filepath.Join(audio.dir, fmt.Sprintf("%s.norm.w64", audio.ID()))
}

func (audio *DolbyAudioStream) Prepare() error {
	return tracerr.Wrap(audio.base.Prepare())
}

func (audio *DolbyAudioStream) Process() error {
	// The file already exists
	if _, err := os.Stat(audio.Path()); !os.IsNotExist(err) {
		return nil
	}

	err := audio.base.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	temp := common.Temp(audio.Path())

	args := make([]interface{}, 0)
	args = append(args, "-drc_scale", "0")

	info := audio.MediaInfo()

	val, exists := info["extra"]
	if exists {
		extra := val.(map[string]interface{})

		val, exists := extra["dialnorm_Average"]
		if exists {
			args = append(args, "-target_level", val.(string))
		}
	}

	args = append(args, "-i", audio.base.Path())
	args = append(args, "-map", fmt.Sprintf("0:%d", audio.base.Index()))
	args = append(args, "-codec", "pcm_f32le")
	args = append(args, "-y", temp)

	_, err = sh.Command(common.FFMPEG, args...).CombinedOutput()
	if err != nil {
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, audio.Path())

	// Cleanup
	err = audio.base.Cleanup()
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *DolbyAudioStream) Cleanup() error {
	err := os.Remove(audio.Path())
	if err != nil {
		return tracerr.Wrap(err)
	}

	return nil
}

func (audio *DolbyAudioStream) Samplerate() int {
	return audio.base.Samplerate()
}
