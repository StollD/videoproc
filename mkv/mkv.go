package mkv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/StollD/videoproc/common"
	"github.com/codeskyblue/go-sh"
	"github.com/natefinch/atomic"
	"github.com/ztrue/tracerr"
)

type MKV struct {
	path    string
	probe   []interface{}
	minfo   []interface{}
	streams []Stream
}

func NewMKV(path string) *MKV {
	return &MKV{path: path, streams: make([]Stream, 0)}
}

func (mkv *MKV) Path() string {
	return mkv.path
}

// Returns the base name of the file
// /home/foo/bar.mkv -> bar
func (mkv *MKV) Name() string {
	base := filepath.Base(mkv.Path())
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return strings.TrimSuffix(name, ".norm")
}

// Uses mkvmerge to normalize the metadata of the file
// (language codes, statistics tags from MakeMKV)
func (mkv *MKV) Normalize(dir string) error {
	out := filepath.Join(dir, fmt.Sprintf("%s.norm.mkv", mkv.Name()))

	// Does the file already exist?
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		mkv.path = out
		return nil
	}

	temp := common.Temp(out)
	_, err := sh.Command(common.MKVMERGE, mkv.Path(), "-o", temp).CombinedOutput()
	if err != nil {
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp, out)
	mkv.path = out

	return nil
}

// Use ffprobe to load information about the MKV file
func (mkv *MKV) Probe() error {
	shell := sh.Command(common.FFPROBE, "-show_streams", "-probesize", "10G", "-analyzeduration", "10G", "-of", "json", mkv.Path())
	shell.Stderr = common.DevNull()

	var probe map[string]interface{}
	err := shell.UnmarshalJSON(&probe)
	if err != nil {
		return tracerr.Wrap(err)
	}

	mkv.probe = probe["streams"].([]interface{})

	var mediainfo map[string]interface{}

	err = sh.Command(common.MEDIAINFO, "--Output=JSON", "-f", mkv.Path()).UnmarshalJSON(&mediainfo)
	if err != nil {
		return tracerr.Wrap(err)
	}

	media := mediainfo["media"].(map[string]interface{})
	mkv.minfo = media["track"].([]interface{})

	return nil
}

// Find a stream using a selector expression
func (mkv *MKV) Stream(sel string) (Stream, error) {
	for _, s := range mkv.probe {
		probe := s.(map[string]interface{})

		index, exists := probe["index"]
		if !exists {
			continue
		}

		info := mkv.minfo[int(index.(float64))+1].(map[string]interface{})

		stream := NewBasicStream(mkv.Path(), probe, info)

		video := stream.Type() == "video"
		audio := stream.Type() == "audio"
		subtitle := stream.Type() == "subtitle"

		// If this is not a video, audio or subtitle stream,
		// it cannot be handled by the program
		if !video && !audio && !subtitle {
			continue
		}

		// Check if this stream matches the selector
		value, err := gval.Evaluate(sel, map[string]interface{}{
			"track": stream.ID(),
			"lang":  stream.Language(),
			"type":  stream.Type(),
		})

		if err != nil {
			return nil, tracerr.Wrap(err)
		}

		if !(value.(bool)) {
			continue
		}

		if video {
			stream = NewBasicVideoStream(stream)
		} else if audio {
			stream = NewBasicAudioStream(stream)
		} else if subtitle {
			stream = NewBasicSubtitleStream(stream)
		}

		return stream, nil
	}

	return nil, nil
}

// Add a stream to the final MKV
func (mkv *MKV) AddStream(stream Stream) {
	mkv.streams = append(mkv.streams, stream)
}

// Run preparation steps on the streams
func (mkv *MKV) Prepare() error {
	for _, s := range mkv.streams {
		err := s.Prepare()
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	return nil
}

// Run processing steps on all streams
func (mkv *MKV) Process() error {
	for _, s := range mkv.streams {
		err := s.Process()
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	return nil
}

// Run cleanup steps on all streams
func (mkv *MKV) Cleanup() error {
	for _, s := range mkv.streams {
		err := s.Cleanup()
		if err != nil {
			return tracerr.Wrap(err)
		}
	}

	return nil
}

func (mkv *MKV) Chapters() (string, error) {
	var video Stream

	for _, s := range mkv.streams {
		if s.Type() != "video" {
			continue
		}

		video = s
		break
	}

	temp := filepath.Join(os.TempDir(), "chapters.txt")

	cmd := sh.Command(common.FFMPEG, "-i", video.Path(), "-f", "ffmetadata", "-")
	cmd.Stderr = common.DevNull()

	out, err := cmd.Output()
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	chapters := []string{}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "title") {
			continue
		}

		chapters = append(chapters, line)
	}

	err = os.WriteFile(temp, []byte(strings.Join(chapters, "\n")), 0644)
	if err != nil {
		return "", tracerr.Wrap(err)
	}

	return temp, nil
}

// Merge all selected and processed streams into a new mkv file
func (mkv *MKV) Merge(dir string) error {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return tracerr.Wrap(err)
	}

	out := filepath.Join(dir, fmt.Sprintf("%s.mkv", mkv.Name()))
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		return nil
	}

	var args []interface{}

	for _, s := range mkv.streams {
		args = append(args, "-itsoffset", fmt.Sprintf("%fs", s.Offset()))
		args = append(args, "-i", s.Path())
	}

	// Chapters
	chapters, err := mkv.Chapters()
	if err != nil {
		return tracerr.Wrap(err)
	}

	args = append(args, "-i", chapters)

	for i, s := range mkv.streams {
		args = append(args, "-map", fmt.Sprintf("%d:%d", i, s.Index()))

		// Unset some metadata
		meta := fmt.Sprintf("-metadata:s:%d", i)
		args = append(args, meta, "title=")
		args = append(args, meta, "SOURCE_ID=")
		args = append(args, meta, "ENCODER=")

		if s.Type() == "video" {
			args = append(args, meta, "language=")
		} else {
			args = append(args, meta, fmt.Sprintf("language=%s", s.Language()))
		}
	}

	args = append(args, "-map_chapters", fmt.Sprintf("%d", len(mkv.streams)))

	temp := common.Temp(out)

	args = append(args, "-codec", "copy")
	args = append(args, "-disposition", "0")
	args = append(args, "-disposition:a:0", "default")
	args = append(args, "-metadata", "title=")
	args = append(args, "-y", temp)

	_, err = sh.Command(common.FFMPEG, args...).CombinedOutput()
	if err != nil {
		return tracerr.Wrap(err)
	}

	temp2 := common.Temp(temp)

	_, err = sh.Command(common.MKVMERGE, temp, "-o", temp2).CombinedOutput()
	if err != nil {
		return tracerr.Wrap(err)
	}

	err = os.Remove(temp)
	if err != nil {
		return tracerr.Wrap(err)
	}

	atomic.ReplaceFile(temp2, out)
	return nil
}
