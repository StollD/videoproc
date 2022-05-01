package common

import "os"

const FFMPEG = "ffmpeg"
const VSPIPE = "vspipe"
const FFPROBE = "ffprobe"
const MKVMERGE = "mkvmerge"
const MEDIAINFO = "mediainfo"

func DevNull() *os.File {
	null, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
	return null
}
