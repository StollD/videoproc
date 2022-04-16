package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/codeskyblue/go-sh"
	"github.com/gosuri/uilive"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

func ProcessStreams(file *VideoFile, paths Paths) error {
	fmt.Printf("Processing %s\n\n", file.Input)

	workdir := filepath.Join(paths.Working, file.Config.Name, file.Name, "process")
	err := os.MkdirAll(workdir, 0755)
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = ProcessVideo(file, paths, workdir)
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("\n")

	for stream := range file.Audio {
		err = ProcessAudio(file, paths, workdir, stream)
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	fmt.Printf("\n")

	for stream := range file.Subtitles {
		err = ProcessSubtitle(file, paths, workdir, stream)
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	fmt.Printf("\n")

	err = ProcessChapters(file, paths, workdir)
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("\n")
	return nil
}

func ProcessVideo(file *VideoFile, paths Paths, workdir string) error {
	ext := filepath.Ext(file.Video)

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	// Run the vapoursynth filter and save the output as tif frames
	if ext == ".vpy" {
		path, err := RunVapoursynth(file, paths, workdir, ui)
		if err != nil {
			return tracerr.Wrap(err)
		}

		file.Video = path
	}

	// Encode the video (either tif frames or mkv) using the provided codec
	path, err := RunVideoEncode(file, paths, workdir, ui)
	if err != nil {
		return tracerr.Wrap(err)
	}

	// Cleanup
	if ext == ".vpy" {
		dir := filepath.Dir(file.Video)
		err = os.RemoveAll(dir)
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	file.Video = path
	return nil
}

func RunVapoursynth(file *VideoFile, paths Paths, workdir string, ui *uilive.Writer) (string, error) {
	frames := filepath.Join(workdir, fmt.Sprintf("%s_frames", file.Name))
	out := filepath.Join(frames, "%09d.tiff")

	name := fmt.Sprintf("%s_video", file.Name)
	mkv := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))

	// The encoded file already exists
	if _, err := os.Stat(mkv); !os.IsNotExist(err) {
		return out, nil
	}

	if paths.VSPipe == "" {
		return "", tracerr.New("vspipe is required for using VapourSynth filters")
	}

	err := os.MkdirAll(frames, 0755)
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	files, err := filepath.Glob(filepath.Join(frames, "*.tiff"))
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	last := -1

	// Determine how many frames have been processed already
	sort.Strings(files)
	if len(files) > 0 {
		lf := filepath.Base(files[len(files)-1])
		ln, err := strconv.ParseInt(strings.TrimSuffix(lf, ".tiff"), 10, 32)
		if err != nil {
			return "", tracerr.Wrap(err)
		}

		last = int(ln)

		if last != len(files)-1 {
			return "", tracerr.Errorf("%s has missing frames", frames)
		}
	}

	ls := fmt.Sprintf("%d", last+1)

	if last >= file.Frames {
		fmt.Fprintf(ui, "  Already filtered video\n")
		return out, nil
	}

	filter := filepath.Base(file.Config.Filter)
	fmt.Fprintf(ui, "  Filtering video using filter %s\n", filter)

	read, write := io.Pipe()
	vspipe := sh.Command(paths.VSPipe, file.Video, "-", "-c", "y4m", "-s", ls)
	cmd := vspipe.Command(paths.FFMPEG, "-progress", "-", "-nostats", "-i", "pipe:", "-f", "image2", "-start_number", ls, out)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		fmt.Fprintf(ui, "  Filtering video using filter %s: %s (Frame: %s; FPS: %s)\n ", filter, data["out_time"], data["frame"], data["fps"])
	})

	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while filtering video\n")
		return "", tracerr.Wrap(err)
	}

	read.Close()
	write.Close()

	fmt.Fprintf(ui, "  Finished filtering video\n")
	return out, nil
}

func RunVideoEncode(file *VideoFile, paths Paths, workdir string, ui *uilive.Writer) (string, error) {
	var args []interface{}

	name := fmt.Sprintf("%s_video", file.Name)
	out := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.mkv", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Fprintf(ui, "  Already processed video\n")
		return out, nil
	}

	args = append(args, "-progress", "-", "-nostats")
	args = append(args, "-r", file.Framerate.String())

	if filepath.Ext(file.Video) == ".tif" {
		args = append(args, "-f", "image2")
	}

	args = append(args, "-i", file.Video)

	// Prepare the encoding parameters
	encargs := file.Config.Video

	// Check if a codec was configured
	if _, found := encargs["codec"]; !found {
		return "", tracerr.New("no video codec configured")
	}

	// If no aspect ratio is set, reuse the original one
	if _, found := encargs["aspect"]; !found {
		encargs["aspect"] = file.Aspect
	}

	for key, val := range encargs {
		args = append(args, fmt.Sprintf("-%s", key), val)
	}

	args = append(args, "-y", temp)

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, args...)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		fmt.Fprintf(ui, "  Processing video using codec %s: %s (Frame: %s; FPS: %s)\n", encargs["codec"], data["out_time"], data["frame"], data["fps"])
	})

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while processing video\n")
		return "", tracerr.Wrap(err)
	}

	read.Close()
	write.Close()

	fmt.Fprintf(ui, "  Finished processing video\n")

	atomic.ReplaceFile(temp, out)
	return out, nil
}

func ProcessAudio(file *VideoFile, paths Paths, workdir string, stream string) error {
	var args []interface{}

	name := fmt.Sprintf("%s_%s", file.Name, stream)
	out := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.mkv", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Already processed audio %s\n", stream)
		file.Audio[stream] = out
		return nil
	}

	// Prepare the encoding parameters
	encargs := file.Config.Audio

	// Check if a codec was configured
	if _, found := encargs["codec"]; !found {
		fmt.Printf("  Error while processing audio %s\n", stream)
		return tracerr.New("no audio codec configured")
	}

	// If the audio is copied, there is nothing to do here
	if encargs["codec"] == "copy" {
		fmt.Printf("  Audio %s does not need processing\n", stream)
		return nil
	}

	// Check if MediaInfo is available
	if paths.MediaInfo == "" {
		fmt.Printf("  Error while processing audio %s\n", stream)
		return tracerr.New("MediaInfo is required to process audio")
	}

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	fmt.Fprintf(ui, "  Probing audio %s\n", stream)

	audio := file.Probe[stream]
	var mediainfo map[string]interface{}

	err := sh.Command(paths.MediaInfo, "--Output=JSON", "-f", file.Audio[stream]).UnmarshalJSON(&mediainfo)
	if err != nil {
		fmt.Fprintf(ui, "  Error while processing audio %s\n", stream)
		return tracerr.Wrap(err)
	}

	args = append(args, "-progress", "-", "-nostats")
	args = append(args, "-drc_scale", "0")

	media := mediainfo["media"].(map[string]interface{})
	tracks := media["track"].([]interface{})

	for _, t := range tracks {
		track := t.(map[string]interface{})

		// We want the audio track
		if track["@type"].(string) != "Audio" {
			continue
		}

		// The interesting stuff is under extra
		extra := track["extra"].(map[string]interface{})
		if dialnorm, found := extra["dialnorm_Average"]; found {
			args = append(args, "-target_level", dialnorm.(string))
		}

		// Apply values here to the codec
		for key, val := range encargs {
			for k2, v2 := range extra {
				val = strings.ReplaceAll(val, fmt.Sprintf("$(%s)$", k2), v2.(string))
			}

			match, err := filepath.Match("$(*)$", val)
			if err != nil || match {
				delete(encargs, key)
			} else {
				encargs[key] = val
			}
		}
	}

	args = append(args, "-i", file.Audio[stream])

	filter := fmt.Sprintf("asetrate=%s*%f,aresample", audio["sample_rate"].(string), file.Speedup())
	args = append(args, "-af", filter)
	args = append(args, "-ar", audio["sample_rate"])

	for key, val := range encargs {
		args = append(args, fmt.Sprintf("-%s", key), val)
	}

	args = append(args, "-y", temp)

	fmt.Fprintf(ui, "  Processing audio %s using codec %s\n", stream, encargs["codec"])

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, args...)
	cmd = FFRedirectProgress(cmd, write)

	go FFReadProgress(read, func(data map[string]string) {
		fmt.Fprintf(ui, "  Processing audio %s using codec %s: %s\n", stream, encargs["codec"], data["out_time"])
	})

	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while processing audio %s\n", stream)
		return tracerr.Wrap(err)
	}

	read.Close()
	write.Close()

	fmt.Fprintf(ui, "  Finished processing audio %s\n", stream)

	atomic.ReplaceFile(temp, out)
	file.Audio[stream] = out
	return nil
}

func ProcessSubtitle(file *VideoFile, paths Paths, workdir string, stream string) error {
	var args []interface{}

	name := fmt.Sprintf("%s_%s", file.Name, stream)
	out := filepath.Join(workdir, fmt.Sprintf("%s.mkv", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.mkv", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Already processed subtitle %s\n", stream)
		file.Subtitles[stream] = out
		return nil
	}

	ui := uilive.New()
	ui.Start()
	defer ui.Stop()

	args = append(args, "-progress", "-", "-nostats")
	args = append(args, "-i", file.Subtitles[stream])
	args = append(args, "-map", "0")
	args = append(args, "-bsf", fmt.Sprintf("setts=TS*%f", file.Slowdown()))
	args = append(args, "-c", "copy")
	args = append(args, "-y", temp)

	fmt.Fprintf(ui, "  Processing subtitle %s\n", stream)

	read, write := io.Pipe()
	cmd := sh.Command(paths.FFMPEG, args...)
	cmd = FFRedirectProgress(cmd, write)
	cmd.ShowCMD = true

	go FFReadProgress(read, func(data map[string]string) {
		fmt.Fprintf(ui, "  Processing subtitle %s: %s\n", stream, data["out_time"])
	})

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(ui, "  Error while processing subtitle %s\n", stream)
		return tracerr.Wrap(err)
	}

	read.Close()
	write.Close()

	fmt.Fprintf(ui, "  Finished processing subtitle %s\n", stream)

	atomic.ReplaceFile(temp, out)
	file.Subtitles[stream] = out
	return nil
}

func ProcessChapters(file *VideoFile, paths Paths, workdir string) error {
	name := fmt.Sprintf("%s_chapters", file.Name)

	out := filepath.Join(workdir, fmt.Sprintf("%s.txt", name))
	temp := filepath.Join(workdir, fmt.Sprintf("%s.temp.txt", name))

	// The file already exists
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Printf("  Already processed chapters\n")
		file.Chapters = out
		return nil
	}

	// Do we have chapters?
	if _, err := os.Stat(file.Chapters); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(file.Chapters)
	if err != nil {
		return tracerr.Wrap(err)
	}

	chapters := []string{}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "START") || strings.HasPrefix(line, "END") {
			split := strings.Split(line, "=")
			val, err := strconv.ParseInt(split[1], 10, 64)
			if err != nil {
				return tracerr.Wrap(err)
			}

			line = fmt.Sprintf("%s=%d", split[0], int(float64(val)*file.Slowdown()))
		}

		if strings.HasPrefix(line, "title") {
			continue
		}

		chapters = append(chapters, line)
	}

	err = os.WriteFile(temp, []byte(strings.Join(chapters, "\n")), 0644)
	if err != nil {
		return tracerr.Wrap(err)
	}

	fmt.Printf("  Finished processing chapters\n")

	atomic.ReplaceFile(temp, out)
	file.Chapters = out
	return nil
}
