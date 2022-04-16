package main

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/codeskyblue/go-sh"
)

func FFRedirectProgress(cmd *sh.Session, writer io.Writer) *sh.Session {
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
	if err == nil {
		cmd.Stderr = null
	}

	cmd.Stdout = writer
	return cmd
}

func FFReadProgress(reader io.Reader, callback func(data map[string]string)) {
	scanner := bufio.NewScanner(reader)

	data := map[string]string{}

	for scanner.Scan() {
		line := scanner.Text()
		split := strings.Split(line, "=")

		if _, found := data[split[0]]; found {
			callback(data)
			data = map[string]string{}
		}

		if split[0] == "progress" && split[1] == "end" {
			break
		}

		data[split[0]] = split[1]
	}
}
