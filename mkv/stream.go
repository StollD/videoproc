package mkv

import (
	"strconv"
)

type Stream interface {
	Probe() map[string]interface{}
	MediaInfo() map[string]interface{}

	ID() string
	Index() int
	Type() string
	Language() string
	Offset() float64

	Path() string

	Prepare() error
	Process() error
	Cleanup() error
}

type BasicStream struct {
	Stream

	path      string
	probe     map[string]interface{}
	mediainfo map[string]interface{}
}

func NewBasicStream(mkv string, probe map[string]interface{}, mediainfo map[string]interface{}) Stream {
	return &BasicStream{
		probe:     probe,
		path:      mkv,
		mediainfo: mediainfo,
	}
}

func (base *BasicStream) Probe() map[string]interface{} {
	return base.probe
}

func (base *BasicStream) MediaInfo() map[string]interface{} {
	return base.mediainfo
}

func (base *BasicStream) Tag(name string) string {
	probe := base.Probe()

	tags, exists := probe["tags"]
	if !exists {
		return ""
	}

	val, exists := (tags.(map[string]interface{}))[name]
	if !exists {
		return ""
	}

	return val.(string)
}

func (base *BasicStream) ID() string {
	return base.Tag("SOURCE_ID")
}

func (base *BasicStream) Type() string {
	probe := base.Probe()

	val, exists := probe["codec_type"]
	if !exists {
		return ""
	}

	return val.(string)
}

func (base *BasicStream) Index() int {
	probe := base.Probe()

	val, exists := probe["index"]
	if !exists {
		return -1
	}

	return int(val.(float64))
}

func (base *BasicStream) Language() string {
	return base.Tag("language")
}

func (base *BasicStream) Offset() float64 {
	probe := base.Probe()

	val, exists := probe["start_time"]
	if !exists {
		return 0
	}

	time, err := strconv.ParseFloat(val.(string), 64)
	if err != nil {
		return 0
	}

	return time
}

func (base *BasicStream) Path() string {
	return base.path
}

func (base *BasicStream) Prepare() error {
	return nil
}

func (base *BasicStream) Process() error {
	return nil
}

func (base *BasicStream) Cleanup() error {
	return nil
}
