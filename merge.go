package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/ztrue/tracerr"
)

func Merge(file Input, paths Paths) error {
	fmt.Printf("Creating %s/%s.mkv\t", file.CFG.Name, file.MKV.Name())

	dir := filepath.Join(paths.Output, file.CFG.Name)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	err = file.MKV.Merge(dir)
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	color.Green("✓")
	return nil
}
