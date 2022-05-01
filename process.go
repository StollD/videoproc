package main

import (
	"fmt"

	"github.com/ztrue/tracerr"
)

func Process(file Input, paths Paths) error {
	fmt.Printf("Processing %s/%s.mkv\n\n", file.CFG.Name, file.MKV.Name())

	err := file.MKV.Process()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("\n")
	return nil
}
