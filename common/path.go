package common

import (
	"path/filepath"
	"strings"
)

func Temp(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".temp" + ext
}
