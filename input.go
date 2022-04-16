package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ztrue/tracerr"
)

func FindInputs(paths Paths) ([]VideoFile, error) {
	// Find entries within the input directory
	subdirs, err := os.ReadDir(paths.Input)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	ret := []VideoFile{}

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
			base := strings.TrimSuffix(file.Name(), ext)

			// We only care about mkv files
			if ext != ".mkv" {
				continue
			}

			video := VideoFile{
				Name:   base,
				Input:  filepath.Join(path, file.Name()),
				Output: filepath.Join(paths.Output, dir.Name(), file.Name()),
				Config: config,

				Frames: -1,

				Audio:     map[string]string{},
				Subtitles: map[string]string{},
				Probe:     map[string]map[string]interface{}{},
			}

			fmt.Printf("  Found input: %s\n", video.Input)
			ret = append(ret, video)
		}

		fmt.Println()
	}

	return ret, nil
}
