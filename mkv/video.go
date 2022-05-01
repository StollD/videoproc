package mkv

import "github.com/ztrue/tracerr"

type VideoStream interface {
	Stream

	Aspect() string
	Framerate() Framerate
	Speedup() float64
}

type BasicVideoStream struct {
	VideoStream

	base Stream
}

func NewBasicVideoStream(base Stream) VideoStream {
	return &BasicVideoStream{
		base: base,
	}
}

func (video *BasicVideoStream) Probe() map[string]interface{} {
	return video.base.Probe()
}

func (video *BasicVideoStream) MediaInfo() map[string]interface{} {
	return video.base.MediaInfo()
}

func (video *BasicVideoStream) ID() string {
	return video.base.ID()
}

func (video *BasicVideoStream) Type() string {
	return video.base.Type()
}

func (video *BasicVideoStream) Index() int {
	return video.base.Index()
}

func (video *BasicVideoStream) Language() string {
	return video.base.Language()
}

func (video *BasicVideoStream) Offset() float64 {
	return video.base.Offset()
}

func (video *BasicVideoStream) Path() string {
	return video.base.Path()
}

func (video *BasicVideoStream) Prepare() error {
	return tracerr.Wrap(video.base.Prepare())
}

func (video *BasicVideoStream) Process() error {
	return tracerr.Wrap(video.base.Process())
}

func (video *BasicVideoStream) Cleanup() error {
	return tracerr.Wrap(video.base.Cleanup())
}

func (video *BasicVideoStream) Aspect() string {
	probe := video.Probe()

	val, exists := probe["display_aspect_ratio"]
	if !exists {
		return ""
	}

	return val.(string)
}

func (video *BasicVideoStream) Framerate() Framerate {
	probe := video.Probe()
	framerate := Framerate{}

	val, exists := probe["avg_frame_rate"]
	if !exists {
		return framerate
	}

	framerate.Parse(val.(string))
	return framerate
}

func (video *BasicVideoStream) Speedup() float64 {
	return 1
}
