package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/codeskyblue/go-sh"
	"github.com/ztrue/tracerr"
)

func main() {
	paths := Paths{}

	// Load paths from the commandline
	flag.StringVar(&paths.Input, "i", "input", "Input directory")
	flag.StringVar(&paths.Config, "c", "config", "Config directory")
	flag.StringVar(&paths.Working, "w", "working", "Working directory")
	flag.StringVar(&paths.Output, "o", "output", "Output directory")
	flag.StringVar(&paths.FFMPEG, "ffmpeg", "ffmpeg", "Path to ffmpeg executable")
	flag.StringVar(&paths.FFPROBE, "ffprobe", "ffprobe", "Path to ffprobe executable")
	flag.StringVar(&paths.VSPipe, "vspipe", "vspipe", "Path to vspipe executable")
	flag.StringVar(&paths.MediaInfo, "mediainfo", "mediainfo", "Path to mediainfo executable")
	flag.StringVar(&paths.MKVPropEdit, "mkvpropedit", "mkvpropedit", "Path to mkvpropedit executable")
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

	// Test ffmpeg
	fmt.Printf("Testing %s: ", paths.FFMPEG)
	_, err = sh.Command(paths.FFMPEG, "-version").CombinedOutput()

	if err != nil {
		fmt.Printf("FAILED\n")
		paths.FFMPEG = ""
	} else {
		fmt.Printf("SUCCESS\n")
	}

	// Test ffprobe
	fmt.Printf("Testing %s: ", paths.FFPROBE)
	_, err = sh.Command(paths.FFPROBE, "-version").CombinedOutput()

	if err != nil {
		fmt.Printf("FAILED\n")
		paths.FFPROBE = ""
	} else {
		fmt.Printf("SUCCESS\n")
	}

	// Test vspipe
	fmt.Printf("Testing %s: ", paths.VSPipe)
	_, err = sh.Command(paths.VSPipe, "--version").CombinedOutput()

	if err != nil {
		fmt.Printf("FAILED\n")
		paths.VSPipe = ""
	} else {
		fmt.Printf("SUCCESS\n")
	}

	// Test mediainfo
	fmt.Printf("Testing %s: ", paths.MediaInfo)
	_, err = sh.Command(paths.MediaInfo, "--Version").CombinedOutput()

	if err != nil {
		fmt.Printf("FAILED\n")
		paths.MediaInfo = ""
	} else {
		fmt.Printf("SUCCESS\n")
	}

	// Test mkvpropedit
	fmt.Printf("Testing %s: ", paths.MKVPropEdit)
	_, err = sh.Command(paths.MKVPropEdit, "--version").CombinedOutput()

	if err != nil {
		fmt.Printf("FAILED\n")
		paths.MKVPropEdit = ""
	} else {
		fmt.Printf("SUCCESS\n")
	}

	// Check if any required tool is missing
	if paths.FFMPEG == "" || paths.FFPROBE == "" {
		fmt.Printf("\nffmpeg and ffprobe are required!")
		os.Exit(1)
	}

	fmt.Printf("\n=====================\n\n")

	// Find input files
	files, err := FindInputs(paths)
	if err != nil {
		tracerr.PrintSourceColor(err)
		os.Exit(1)
	}

	fmt.Printf("=====================\n\n")

	// Extract streams from inputs
	for i := 0; i < len(files); i++ {
		err = ExtractStreams(&files[i], paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("=====================\n\n")

	// Process streams
	for i := 0; i < len(files); i++ {
		err = ProcessStreams(&files[i], paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("=====================\n\n")

	// Merge streams
	for i := 0; i < len(files); i++ {
		err = MergeStreams(&files[i], paths)
		if err != nil {
			tracerr.PrintSourceColor(err)
			os.Exit(1)
		}
	}

	fmt.Printf("=====================\n\n")
}
