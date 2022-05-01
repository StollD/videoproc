package main

import (
	"path/filepath"

	"github.com/ztrue/tracerr"
)

type Paths struct {
	Input   string
	Config  string
	Working string
	Output  string
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

	new.Input = indir
	new.Config = confdir
	new.Working = workdir
	new.Output = outdir

	return new, nil
}
