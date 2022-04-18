package main

import (
	"os"
	"path/filepath"

	"github.com/ztrue/tracerr"
)

type Paths struct {
	Input   string
	Config  string
	Working string
	Output  string

	FFMPEG    string
	FFPROBE   string
	VSPipe    string
	MediaInfo string
	MKVMerge  string
}

func (paths Paths) Abs() (Paths, error) {
	new := Paths{}

	indir, err := filepath.Abs(paths.Input)
	if err != nil {
		return new, tracerr.Wrap(err)
	}

	confdir, err := filepath.Abs(paths.Config)
	if err != nil {
		return new, tracerr.Wrap(err)
	}

	workdir, err := filepath.Abs(paths.Working)
	if err != nil {
		return new, tracerr.Wrap(err)
	}

	outdir, err := filepath.Abs(paths.Output)
	if err != nil {
		return new, tracerr.Wrap(err)
	}

	ffmpeg := paths.FFMPEG
	if _, err := os.Stat(ffmpeg); !os.IsNotExist(err) {
		ffmpeg, err = filepath.Abs(ffmpeg)
		if err != nil {
			return new, tracerr.Wrap(err)
		}
	}

	ffprobe := paths.FFPROBE
	if _, err := os.Stat(ffprobe); !os.IsNotExist(err) {
		ffprobe, err = filepath.Abs(ffprobe)
		if err != nil {
			return new, tracerr.Wrap(err)
		}
	}

	vspipe := paths.VSPipe
	if _, err := os.Stat(vspipe); !os.IsNotExist(err) {
		vspipe, err = filepath.Abs(vspipe)
		if err != nil {
			return new, tracerr.Wrap(err)
		}
	}

	mediainfo := paths.MediaInfo
	if _, err := os.Stat(mediainfo); !os.IsNotExist(err) {
		mediainfo, err = filepath.Abs(mediainfo)
		if err != nil {
			return new, tracerr.Wrap(err)
		}
	}

	mkvmerge := paths.MKVMerge
	if _, err := os.Stat(mkvmerge); !os.IsNotExist(err) {
		mkvmerge, err = filepath.Abs(mkvmerge)
		if err != nil {
			return new, tracerr.Wrap(err)
		}
	}

	new.Input = indir
	new.Config = confdir
	new.Working = workdir
	new.Output = outdir

	new.FFMPEG = ffmpeg
	new.FFPROBE = ffprobe
	new.VSPipe = vspipe
	new.MediaInfo = mediainfo
	new.MKVMerge = mkvmerge

	return new, nil
}
