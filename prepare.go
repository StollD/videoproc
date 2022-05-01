package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/StollD/videoproc/filters"
	"github.com/StollD/videoproc/mkv"
	"github.com/fatih/color"
	"github.com/ztrue/tracerr"
)

func Prepare(file Input, paths Paths) error {
	fmt.Printf("Preparing %s/%s.mkv\n\n", file.CFG.Name, file.MKV.Name())

	dir := filepath.Join(paths.Working, file.CFG.Name, file.MKV.Name(), "extract")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return tracerr.Wrap(err)
	}

	var streams []string

	keys := make([]string, 0)
	for key := range file.CFG.Streams {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	// Find the stream config that matches the current file
	for _, key := range keys {
		val := file.CFG.Streams[key]

		match, err := filepath.Match(key, file.MKV.Name())
		if err != nil || !match {
			continue
		}

		vid := []string{"type == \"video\""}
		streams = append(vid, val...)
	}

	var vcodec map[string]string

	keys = make([]string, 0)
	for key := range file.CFG.Video {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	// Find the stream config that matches the current file
	for _, key := range keys {
		val := file.CFG.Video[key]

		match, err := filepath.Match(key, file.MKV.Name())
		if err != nil || !match {
			continue
		}

		vcodec = val
	}

	var acodec map[string]string

	keys = make([]string, 0)
	for key := range file.CFG.Audio {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	// Find the stream config that matches the current file
	for _, key := range keys {
		val := file.CFG.Audio[key]

		match, err := filepath.Match(key, file.MKV.Name())
		if err != nil || !match {
			continue
		}

		acodec = val
	}

	fmt.Printf("  Normalizing metadata\t\t")

	err = file.MKV.Normalize(dir)
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	color.Green("✓")
	fmt.Printf("  Probing input file\t\t")

	err = file.MKV.Probe()
	if err != nil {
		color.Red("✗")
		return tracerr.Wrap(err)
	}

	color.Green("✓")
	fmt.Printf("\n")

	// Save the video stream for later
	var video mkv.VideoStream = nil

	for _, s := range streams {
		stream, err := file.MKV.Stream(s)
		if err != nil {
			return tracerr.Wrap(err)
		}

		if stream == nil {
			color.Yellow("  Could not find match for " + s)
			continue
		}

		if stream.Type() == "video" && video == nil {
			video = stream.(mkv.VideoStream)

			// ADD VIDEO FILTERS
			video = filters.ExtractVideo(video, dir)

			if file.CFG.Filter != "" {
				video = filters.VapoursynthFilter(video, dir, file.CFG.Filter)
			}

			// Encode video with configured codec
			video = filters.EncodeVideo(video, dir, vcodec)

			// Ajust framerate
			if file.CFG.Framerate != "" {
				video = filters.ChangeVideoFramerate(video, dir, file.CFG.Framerate)
			}

			file.MKV.AddStream(video)
		} else if stream.Type() == "audio" {
			audio := stream.(mkv.AudioStream)

			// ADD AUDIO FILTERS
			audio = filters.ExtractAudio(audio, dir)

			codec, exists := acodec["codec"]
			if codec != "copy" && exists {
				// Normalize Dolby audio (remove drc and apply dialnorm)
				audio = filters.NormalizeDolby(audio, dir)

				// If the speed of the video changed, sync the audio
				if math.Abs(video.Speedup()-1) > 0.0001 {
					audio = filters.SyncAudio(audio, video, dir, acodec)
				}

				// Encode audio with configured codec
				audio = filters.EncodeAudio(audio, dir, acodec)
			}

			file.MKV.AddStream(audio)
		} else if stream.Type() == "subtitle" {
			sub := stream.(mkv.SubtitleStream)

			// ADD SUBTITLE FILTERS
			sub = filters.ExtractSubtitle(sub, dir)

			// If the speed of the video changed, sync the subtitles
			if math.Abs(video.Speedup()-1) > 0.0001 {
				sub = filters.SyncSubtitle(sub, video, dir)
			}

			file.MKV.AddStream(sub)
		} else {
			continue
		}

		fmt.Printf("  Selected stream %s (%s, %s)\n", stream.ID(), stream.Language(), stream.Type())
	}

	fmt.Printf("\n")

	// Run preparation steps
	err = file.MKV.Prepare()
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("\n")
	return nil
}
