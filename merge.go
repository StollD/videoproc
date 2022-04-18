package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeskyblue/go-sh"
	"github.com/gosuri/uilive"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

func MergeStreams(file *VideoFile, paths Paths) error {
	fmt.Printf("Assembling %s\n\n", file.Output)

	err := os.MkdirAll(filepath.Dir(file.Output), 0755)
	if err != nil {
		return tracerr.Wrap(err)
	}

	if _, err := os.Stat(file.Output); !os.IsNotExist(err) {
		fmt.Printf("  Already assembled %s\n\n", file.Output)
		return nil
	}

	ui := uilive.New()
	ui.Start()

	var args []interface{}

	args = append(args, "-progress", "-", "-nostats")

	// Video
	args = append(args, "-itsoffset", fmt.Sprintf("%fs", file.Offsets[file.VideoID]*file.Slowdown()))
	args = append(args, "-i", file.Video)

	for _, stream := range file.Streams {
		args = append(args, "-itsoffset", fmt.Sprintf("%fs", file.Offsets[stream]*file.Slowdown()))

		if val, exists := file.Audio[stream]; exists {
			args = append(args, "-i", val)
		}

		if val, exists := file.Subtitles[stream]; exists {
			args = append(args, "-i", val)
		}
	}

	// Chapters
	if _, err := os.Stat(file.Chapters); !os.IsNotExist(err) {
		args = append(args, "-i", file.Chapters)
	}

	// Maps and Metadata
	count := 1 + len(file.Audio) + len(file.Subtitles)
	for i := 0; i < count; i++ {
		args = append(args, "-map", fmt.Sprintf("%d", i))
		args = append(args, fmt.Sprintf("-metadata:s:%d", i), "title=")
		args = append(args, fmt.Sprintf("-metadata:s:%d", i), "ENCODER=")
	}

	// Map chapters
	if _, err := os.Stat(file.Chapters); !os.IsNotExist(err) {
		args = append(args, "-map_chapters", fmt.Sprintf("%d", count))
	}

	// Output
	name := strings.TrimSuffix(filepath.Base(file.Output), filepath.Ext(file.Output))
	temp := filepath.Join(filepath.Dir(file.Output), fmt.Sprintf("%s.temp.mkv", name))
	temp2 := filepath.Join(filepath.Dir(file.Output), fmt.Sprintf("%s.temp2.mkv", name))

	args = append(args, "-codec", "copy")
	args = append(args, "-disposition", "0")
	args = append(args, "-disposition:a:0", "default")
	args = append(args, "-metadata", "title=")
	args = append(args, "-y", temp)

	fmt.Fprintf(ui, "  Creating %s\n", file.Output)

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, args...)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		frame, hasFrame := data["frame"]
		fps, hasFPS := data["fps"]

		if !hasFrame || !hasFPS {
			return
		}

		fmt.Fprintf(ui, "  Creating %s (Frame: %s; FPS: %s)\n", file.Output, frame, fps)
	})

	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while creating %s\n", file.Output)
		ui.Stop()

		return tracerr.Wrap(err)
	}

	fmt.Fprintf(ui, "  Updating metadata of %s\n", file.Output)

	_, err = sh.Command(paths.MKVMerge, temp, "-o", temp2).CombinedOutput()
	if err != nil {
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp2, temp)
	atomic.ReplaceFile(temp, file.Output)

	fmt.Fprintf(ui, "  Finished creating %s\n", file.Output)
	ui.Stop()
	fmt.Printf("\n")
	return nil
}
