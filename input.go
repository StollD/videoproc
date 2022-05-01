package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/StollD/videoproc/mkv"
	"github.com/ztrue/tracerr"
)

type Input struct {
	MKV *mkv.MKV
	CFG Config
}

func FindInputs(paths Paths) ([]Input, error) {
	// Find entries within the input directory
	subdirs, err := os.ReadDir(paths.Input)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	ret := []Input{}

	if len(subdirs) == 0 {
		fmt.Printf("No inputs found\n\n")
		return ret, nil
	}

	for _, dir := range subdirs {
		// We only care about directories
		if !dir.IsDir() {
			continue
		}

		// Load the config based on the directory name
		config, err := LoadConfig(paths, dir.Name())
		if err != nil {
			return nil, tracerr.Wrap(err)
		}

		fmt.Printf("Found config: %s\n\n", dir.Name())
		path := filepath.Join(paths.Input, dir.Name())

		// Load all entries within this directory
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, tracerr.Wrap(err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			ext := filepath.Ext(file.Name())

			// We only care about mkv files
			if ext != ".mkv" {
				continue
			}

			mkv := mkv.NewMKV(filepath.Join(path, file.Name()))
			video := Input{
				MKV: mkv,
				CFG: config,
			}

			fmt.Printf("  Found input: %s/%s\n", config.Name, file.Name())
			ret = append(ret, video)
		}

		fmt.Printf("\n")
	}

	return ret, nil
}
