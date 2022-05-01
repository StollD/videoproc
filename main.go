package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/StollD/videoproc/common"
	"github.com/codeskyblue/go-sh"
	"github.com/fatih/color"
	"github.com/ztrue/tracerr"
)

func main() {
	paths := Paths{}

	// Load paths from the commandline
	flag.StringVar(&paths.Input, "i", "input", "Input directory")
	flag.StringVar(&paths.Config, "c", "config", "Config directory")
	flag.StringVar(&paths.Working, "w", "working", "Working directory")
	flag.StringVar(&paths.Output, "o", "output", "Output directory")
	flag.Parse()

	// Check if input directories exist
	if stat, err := os.Stat(paths.Input); err != nil || !stat.IsDir() {
		fmt.Printf("%s is not a directory!\n", paths.Input)
		os.Exit(1)
	}
	if stat, err := os.Stat(paths.Config); err != nil || !stat.IsDir() {
		fmt.Printf("%s is not a directory!\n", paths.Config)
		os.Exit(1)
	}

	// Create output directories
	if stat, err := os.Stat(paths.Working); err != nil || !stat.IsDir() {
		err = os.MkdirAll(paths.Working, 0755)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}
	if stat, err := os.Stat(paths.Output); err != nil || !stat.IsDir() {
		err = os.MkdirAll(paths.Output, 0755)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	// Make paths absolute
	paths, err := paths.Abs()
	if err != nil {
		tracerr.PrintSourceColor(err)
		os.Exit(1)
	}

	fmt.Printf("\n=====================\n\n")

	failed := false

	// Test ffmpeg
	fmt.Printf("Testing ffmpeg\t\t")
	_, err = sh.Command(common.FFMPEG, "-version").CombinedOutput()

	if err != nil {
		color.Red("✗")
		failed = true
	} else {
		color.Green("✓")
	}

	// Test ffprobe
	fmt.Printf("Testing ffprobe\t\t")
	_, err = sh.Command(common.FFPROBE, "-version").CombinedOutput()

	if err != nil {
		color.Red("✗")
		failed = true
	} else {
		color.Green("✓")
	}

	// Test vspipe
	fmt.Printf("Testing vspipe\t\t")
	_, err = sh.Command(common.VSPIPE, "--version").CombinedOutput()

	if err != nil {
		color.Red("✗")
		failed = true
	} else {
		color.Green("✓")
	}

	// Test mediainfo
	fmt.Printf("Testing mediainfo\t")
	_, err = sh.Command(common.MEDIAINFO, "--Version").CombinedOutput()

	if err != nil {
		color.Red("✗")
		failed = true
	} else {
		color.Green("✓")
	}

	// Test mkvmerge
	fmt.Printf("Testing mkvmerge\t")
	_, err = sh.Command(common.MKVMERGE, "--version").CombinedOutput()

	if err != nil {
		color.Red("✗")
		failed = true
	} else {
		color.Green("✓")
	}

	// Check if any required tool is missing
	if failed {
		fmt.Printf("\nYou have missing programs!\n")
	}

	fmt.Printf("\n=====================\n\n")

	if failed {
		os.Exit(1)
	}

	// Find input files
	files, err := FindInputs(paths)
	if err != nil {
		tracerr.PrintSourceColor(err)
		os.Exit(1)
	}

	fmt.Printf("=====================\n\n")

	// Prepare streams
	for _, file := range files {
		err = Prepare(file, paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("=====================\n\n")

	// Process streams
	for _, file := range files {
		err = Process(file, paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("\n=====================\n\n")

	// Merge streams
	for _, file := range files {
		err = Merge(file, paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("\n=====================\n\n")
}
