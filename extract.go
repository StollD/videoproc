package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/codeskyblue/go-sh"
	"github.com/gosuri/uilive"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

func ExtractStreams(file *VideoFile, paths Paths) error {
	fmt.Printf("Extracting %s\n\n", file.Input)

	workdir := filepath.Join(paths.Working, file.Config.Name, file.Name, "extract")
	err := os.MkdirAll(workdir, 0755)
	if err != nil {
		return tracerr.Wrap(err)
	}

	for key, val := range file.Config.Streams {
		match, err := filepath.Match(key, file.Name)
		if err != nil || !match {
			continue
		}

		file.Streams = val
	}

	input, err := NormalizeMetadata(file, paths, workdir)
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Probing input file\n\n")

	cmd := sh.Command(paths.FFPROBE, "-show_streams", "-probesize", "10G", "-analyzeduration", "10G", "-of", "json", input)
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
	if err == nil {
		cmd.Stderr = null
	}

	var probe map[string]interface{}
	err = cmd.UnmarshalJSON(&probe)
	if err != nil {
		return tracerr.Wrap(err)
	}

	streams := make([]string, len(file.Streams)+1)
	input_streams := probe["streams"].([]interface{})

	// Extract tracks
	for _, s := range input_streams {
		stream := s.(map[string]interface{})

		index := int(stream["index"].(float64))
		codec_type := stream["codec_type"].(string)
		tags := stream["tags"].(map[string]interface{})

		language := ""
		if val, exists := tags["language"]; exists {
			language = val.(string)
		}

		track := ""
		if val, exists := tags["SOURCE_ID"]; exists {
			track = val.(string)
		}

		video := file.Video == "" && codec_type == "video"
		audio := codec_type == "audio"
		subtitle := codec_type == "subtitle"

		position := -1

		for pos, eval := range file.Streams {
			value, err := gval.Evaluate(eval, map[string]interface{}{
				"track": track,
				"lang":  language,
				"index": index,
			})

			if err != nil {
				return tracerr.Wrap(err)
			}

			if !(value.(bool)) {
				continue
			}

			position = pos + 1
			break
		}

		if video {
			position = 0
		}

		if position == -1 {
			continue
		}

		path, err := ExtractStream(file, paths, workdir, input, index, track)
		if err != nil {
			return tracerr.Wrap(err)
		}

		start, err := strconv.ParseFloat(stream["start_time"].(string), 64)
		if err != nil {
			return tracerr.Wrap(err)
		}

		streams[position] = track
		file.Offsets[track] = start

		if video {
			file.Video = path
			file.VideoID = track

			file.Aspect = stream["display_aspect_ratio"].(string)
			file.OriginalFramerate.Parse(stream["avg_frame_rate"].(string))
		}

		if audio {
			file.Audio[track] = path
		}

		if subtitle {
			file.Subtitles[track] = path
		}
	}

	file.Streams = streams

	fmt.Printf("\n")

	path, err := ExtractChapters(file, paths, workdir, input)
	if err != nil {
		return tracerr.Wrap(err)
	}

	file.Chapters = path
	fmt.Printf("\n")

	// Create a filter for the video, if neccessary
	if file.Config.Filter != "" {
		path, err = MakeFilter(file, workdir)
		if err != nil {
			return tracerr.Wrap(err)
		}

		file.Video = path

		frames, framerate, err := VapoursynthInfo(file, paths)
		if err != nil {
			return tracerr.Wrap(err)
		}

		file.Frames = frames
		file.OriginalFramerate.Parse(framerate)

		fmt.Printf("\n")
	}

	// Determine new framerate
	if file.Config.Framerate != "" {
		file.Framerate.Parse(file.Config.Framerate)
	} else {
		file.Framerate = file.OriginalFramerate
	}

	return nil
}

func NormalizeMetadata(file *VideoFile, paths Paths, workdir string) (string, error) {
	out := filepath.Join(workdir, fmt.Sprintf("%s.norm.mkv", file.Name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.norm.temp.mkv", file.Name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Already normalized metadata\n")
		return out, nil
	}

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	fmt.Fprintf(ui, "  Normalizing metadata\n")

	_, err := sh.Command(paths.MKVMerge, file.Input, "-o", temp).CombinedOutput()
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	fmt.Fprintf(ui, "  Normalized metadata\n")

	atomic.ReplaceFile(temp, out)
	return out, nil
}

func ExtractStream(file *VideoFile, paths Paths, workdir string, input string, index int, stream string) (string, error) {
	name := fmt.Sprintf("%s_%s", file.Name, stream)

	out := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.mkv", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Stream %s already extracted\n", stream)
		return out, nil
	}

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	fmt.Fprintf(ui, "  Extracting stream %s\n", stream)

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, "-progress", "-", "-nostats", "-i", input, "-map", fmt.Sprintf("0:%d", index), "-c", "copy", "-y", temp)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		frame, hasFrame := data["frame"]
		fps, hasFPS := data["fps"]

		if !hasFrame || !hasFPS {
			return
		}

		fmt.Fprintf(ui, "  Extracting stream %s (Frame: %s; FPS: %s)\n", stream, frame, fps)
	})

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while extracting stream %s\n", stream)
		return "", tracerr.Wrap(err)
	}

	fmt.Fprintf(ui, "  Extracted stream %s\n", stream)

	atomic.ReplaceFile(temp, out)
	return out, nil
}

func MakeFilter(file *VideoFile, workdir string) (string, error) {
	name := fmt.Sprintf("%s_%s", file.Name, "filter")
	ext := filepath.Ext(file.Config.Filter)

	out := filepath.Join(workdir, name+ext)
	temp := filepath.Join(workdir, name+".temp"+ext)

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Filter already created\n")
		return out, nil
	}

	filter, err := os.ReadFile(file.Config.Filter)
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	new := string(filter)
	new = strings.ReplaceAll(new, "$(video)$", file.Video)
	new = strings.ReplaceAll(new, "$(filter)$", file.Config.Filter)

	err = os.WriteFile(temp, []byte(new), 0644)
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	fmt.Printf("  Created filter based on %s\n", filepath.Base(file.Config.Filter))

	atomic.ReplaceFile(temp, out)
	return out, nil
}

func VapoursynthInfo(file *VideoFile, paths Paths) (int, string, error) {
	fmt.Printf("  Probing VapourSynth for information\n")

	if paths.VSPipe == "" {
		return 0, "", tracerr.New("vspipe is required for using VapourSynth filters")
	}

	vs, err := sh.Command(paths.VSPipe, file.Video, "-i").Output()
	if err != nil {
		return 0, "", tracerr.Wrap(err)
	}

	frames := 0
	framerate := ""

	lines := strings.Split(string(vs), "\n")
	for _, l := range lines {
		split := strings.Split(l, ":")

		if split[0] == "Frames" {
			new, err := strconv.ParseInt(strings.Trim(split[1], " "), 10, 32)
			if err != nil {
				return 0, "", tracerr.Wrap(err)
			}

			frames = int(new)
		}

		if split[0] == "FPS" {
			framerate = strings.Split(split[1], " ")[0]
		}
	}

	return frames, framerate, nil
}

func ExtractChapters(file *VideoFile, paths Paths, workdir string, input string) (string, error) {
	name := fmt.Sprintf("%s_chapters", file.Name)

	out := filepath.Join(workdir, fmt.Sprintf("%s.txt", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.txt", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Chapters already extracted\n")
		return out, nil
	}

	// Extract chapters
	_, err := sh.Command(paths.FFMPEG, "-i", input, "-f", "ffmetadata", out).CombinedOutput()
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	fmt.Printf("  Extracted chapters\n")

	atomic.ReplaceFile(temp, out)
	return out, nil
}
