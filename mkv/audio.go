package mkv

import (
	"strconv"

	"github.com/ztrue/tracerr"
)

type AudioStream interface {
	Stream

	Samplerate() int
}

type BasicAudioStream struct {
	AudioStream

	base Stream
}

func NewBasicAudioStream(base Stream) AudioStream {
	return &BasicAudioStream{base: base}
}

func (audio *BasicAudioStream) Probe() map[string]interface{} {
	return audio.base.Probe()
}

func (audio *BasicAudioStream) MediaInfo() map[string]interface{} {
	return audio.base.MediaInfo()
}

func (audio *BasicAudioStream) ID() string {
	return audio.base.ID()
}

func (audio *BasicAudioStream) Type() string {
	return audio.base.Type()
}

func (audio *BasicAudioStream) Index() int {
	return audio.base.Index()
}

func (audio *BasicAudioStream) Language() string {
	return audio.base.Language()
}

func (audio *BasicAudioStream) Offset() float64 {
	return audio.base.Offset()
}

func (audio *BasicAudioStream) Path() string {
	return audio.base.Path()
}

func (audio *BasicAudioStream) Prepare() error {
	return tracerr.Wrap(audio.base.Prepare())
}

func (audio *BasicAudioStream) Process() error {
	return tracerr.Wrap(audio.base.Process())
}

func (audio *BasicAudioStream) Cleanup() error {
	return tracerr.Wrap(audio.base.Cleanup())
}

func (audio *BasicAudioStream) Samplerate() int {
	info := audio.MediaInfo()

	val, exists := info["SamplingRate"]
	if !exists {
		return -1
	}

	rate, err := strconv.ParseInt(val.(string), 10, 32)
	if err != nil {
		return -1
	}

	return int(rate)
}
