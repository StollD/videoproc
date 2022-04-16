package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	var streams StreamConfig
	for key, val := range file.Config.Streams {
		match, err := filepath.Match(key, file.Name)
		if err != nil || !match {
			continue
		}

		streams = val
	}

	cmd := sh.Command(paths.FFPROBE, "-show_streams", "-of", "json", file.Input)
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
	if err == nil {
		cmd.Stderr = null
	}

	var probe map[string]interface{}
	err = cmd.UnmarshalJSON(&probe)
	if err != nil {
		return tracerr.Wrap(err)
	}

	input_streams := probe["streams"].([]interface{})

	// Extract tracks
	for _, s := range input_streams {
		stream := s.(map[string]interface{})

		tags := stream["tags"].(map[string]interface{})

		// Search for the SOURCE_ID tag, and check its value
		for tag, val := range tags {
			if !strings.HasPrefix(tag, "SOURCE_ID") {
				continue
			}

			track := val.(string)

			video := false
			if file.Video == "" && stream["codec_type"].(string) == "video" {
				video = true
			}

			audio := false
			for _, a := range streams.Audio {
				if a != track {
					continue
				}

				audio = true
				break
			}

			subtitle := false
			for _, s := range streams.Subtitles {
				if s != track {
					continue
				}

				subtitle = true
				break
			}

			if !video && !audio && !subtitle {
				continue
			}

			sel := fmt.Sprintf("m:%s:%s", tag, track)
			path, err := ExtractStream(file, paths, workdir, sel, track)
			if err != nil {
				return tracerr.Wrap(err)
			}

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

			file.Probe[track] = stream
		}
	}

	fmt.Printf("\n")

	path, err := ExtractChapters(file, paths, workdir)
	if err != nil {
		return tracerr.Wrap(err)
	}

	file.Chapters = path

	// Create a filter for the video, if neccessary
	if file.Config.Filter == "" {
		return nil
	}

	path, err = MakeFilter(file, workdir)
	if err != nil {
		return tracerr.Wrap(err)
	}

	file.Video = path
	fmt.Printf("\n")

	frames, framerate, err := VapoursynthInfo(file, paths)
	if err != nil {
		return tracerr.Wrap(err)
	}

	file.Frames = frames
	file.OriginalFramerate.Parse(framerate)

	// Determine new framerate
	if file.Config.Framerate != "" {
		file.Framerate.Parse(file.Config.Framerate)
	} else {
		file.Framerate = file.OriginalFramerate
	}

	fmt.Printf("\n")
	return nil
}

func ExtractStream(file *VideoFile, paths Paths, workdir string, sel string, stream string) (string, error) {
	sel = fmt.Sprintf("0:%s", sel)
	name := fmt.Sprintf("%s_%s", file.Name, stream)

	out := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.mkv", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Stream %s already extracted\n", sel)
		return out, nil
	}

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, "-progress", "-", "-nostats", "-i", file.Input, "-map", sel, "-c", "copy", "-y", temp)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		fmt.Fprintf(ui, "  Extracting stream %s (Frame: %s; FPS: %s)\n", sel, data["frame"], data["fps"])
	})

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while extracting stream %s\n", sel)
		return "", tracerr.Wrap(err)
	}

	fmt.Fprintf(ui, "  Extracted stream %s\n", sel)

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

func ExtractChapters(file *VideoFile, paths Paths, workdir string) (string, error) {
	name := fmt.Sprintf("%s_chapters", file.Name)

	out := filepath.Join(workdir, fmt.Sprintf("%s.txt", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.txt", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Chapters already extracted\n")
		return out, nil
	}

	// Extract chapters
	_, err := sh.Command(paths.FFMPEG, "-i", file.Input, "-f", "ffmetadata", out).CombinedOutput()
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	fmt.Printf("  Extracted chapters\n")

	atomic.ReplaceFile(temp, out)
	return out, nil
}
