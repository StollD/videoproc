package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
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

	offsets := map[string]float64{}
	video := file.Probe[file.VideoID]
	minOff := float64(0)

	// Find offsets of the streams relative to the video
	for track, stream := range file.Probe {
		fmt.Printf("  Probing stream %s\n", track)

		videoStart, err := strconv.ParseFloat(video["start_time"].(string), 64)
		if err != nil {
			return tracerr.Wrap(err)
		}

		streamStart, err := strconv.ParseFloat(stream["start_time"].(string), 64)
		if err != nil {
			return tracerr.Wrap(err)
		}

		diff := streamStart - videoStart
		diff = math.Round(diff * 1000 * file.Slowdown())

		if diff < minOff {
			minOff = diff
		}

		offsets[track] = diff
	}

	// Make sure that the final video starts at 0 (no negative offsets)
	for off := range offsets {
		offsets[off] += -minOff
	}

	fmt.Printf("\n")

	ui := uilive.New()
	ui.Start()

	var args []interface{}

	args = append(args, "-progress", "-", "-nostats")

	// Video
	args = append(args, "-itsoffset", fmt.Sprintf("%fms", offsets[file.VideoID]))
	args = append(args, "-i", file.Video)

	// Audio
	for audio := range file.Audio {
		args = append(args, "-itsoffset", fmt.Sprintf("%fms", offsets[audio]))
		args = append(args, "-i", file.Audio[audio])
	}

	// Subtitles
	for sub := range file.Subtitles {
		args = append(args, "-itsoffset", fmt.Sprintf("%fms", offsets[sub]))
		args = append(args, "-i", file.Subtitles[sub])
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
	}

	// Map chapters
	if _, err := os.Stat(file.Chapters); !os.IsNotExist(err) {
		args = append(args, "-map_chapters", fmt.Sprintf("%d", count))
	}

	// Output
	name := strings.TrimSuffix(filepath.Base(file.Output), filepath.Ext(file.Output))
	temp := filepath.Join(filepath.Dir(file.Output), fmt.Sprintf("%s.temp.mkv", name))

	args = append(args, "-copyts")
	args = append(args, "-codec", "copy")
	args = append(args, "-disposition", "0")
	args = append(args, "-disposition:a:0", "default")
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

	if paths.MKVPropEdit != "" {
		fmt.Fprintf(ui, "  Updating metadata of %s\n", file.Output)

		sh.Command(paths.MKVPropEdit, "--delete-track-statistics-tags", temp).CombinedOutput()
		sh.Command(paths.MKVPropEdit, "--add-track-statistics-tags", temp).CombinedOutput()
	}

	atomic.ReplaceFile(temp, file.Output)

	fmt.Fprintf(ui, "  Finished creating %s\n", file.Output)
	ui.Stop()
	fmt.Printf("\n")
	return nil
}
