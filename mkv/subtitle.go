package mkv

import "github.com/ztrue/tracerr"

type SubtitleStream interface {
	Stream
}

type BasicSubtitleStream struct {
	SubtitleStream

	base Stream
}

func NewBasicSubtitleStream(base Stream) SubtitleStream {
	return &BasicSubtitleStream{
		base: base,
	}
}

func (sub *BasicSubtitleStream) Probe() map[string]interface{} {
	return sub.base.Probe()
}

func (sub *BasicSubtitleStream) MediaInfo() map[string]interface{} {
	return sub.base.MediaInfo()
}

func (sub *BasicSubtitleStream) ID() string {
	return sub.base.ID()
}

func (sub *BasicSubtitleStream) Type() string {
	return sub.base.Type()
}

func (sub *BasicSubtitleStream) Index() int {
	return sub.base.Index()
}

func (sub *BasicSubtitleStream) Language() string {
	return sub.base.Language()
}

func (sub *BasicSubtitleStream) Offset() float64 {
	return sub.base.Offset()
}

func (sub *BasicSubtitleStream) Path() string {
	return sub.base.Path()
}

func (sub *BasicSubtitleStream) Prepare() error {
	return tracerr.Wrap(sub.base.Prepare())
}

func (sub *BasicSubtitleStream) Process() error {
	return tracerr.Wrap(sub.base.Process())
}

func (sub *BasicSubtitleStream) Cleanup() error {
	return tracerr.Wrap(sub.base.Cleanup())
}
