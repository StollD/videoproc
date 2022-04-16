package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ztrue/tracerr"
)

type Framerate struct {
	Numerator   int
	Denominator int
}

type VideoFile struct {
	Name   string
	Input  string
	Output string

	Config  VideoConfig
	Streams StreamConfig

	Video     string
	Audio     map[string]string
	Subtitles map[string]string
	Chapters  string

	VideoID string
	Offsets map[string]float64

	Frames            int
	Framerate         Framerate
	OriginalFramerate Framerate

	Aspect string
}

func (framerate *Framerate) Parse(raw string) error {
	if strings.Contains(raw, "/") {
		split := strings.Split(raw, "/")

		num, err := strconv.ParseInt(split[0], 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		denom, err := strconv.ParseInt(split[1], 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		framerate.Numerator = int(num)
		framerate.Denominator = int(denom)
	} else {
		num, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		framerate.Numerator = int(num)
		framerate.Denominator = 1
	}

	return nil
}

func (framerate Framerate) String() string {
	return fmt.Sprintf("%d/%d", framerate.Numerator, framerate.Denominator)
}

func (framerate Framerate) Value() float64 {
	return float64(framerate.Numerator) / float64(framerate.Denominator)
}

func (video VideoFile) Speedup() float64 {
	return video.Framerate.Value() / video.OriginalFramerate.Value()
}

func (video VideoFile) Slowdown() float64 {
	return 1 / video.Speedup()
}
