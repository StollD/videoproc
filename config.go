package main

import (
	"os"
	"path/filepath"

	"github.com/ztrue/tracerr"
	"gopkg.in/yaml.v2"
)

type StreamConfig struct {
	Audio     []string
	Subtitles []string
}

type VideoConfig struct {
	Name string `yaml:"-"`

	Filter    string
	Framerate string

	// Codecs
	Video map[string]string
	Audio map[string]string

	// Streams to copy
	Streams map[string]StreamConfig
}

func LoadConfig(paths Paths, config string) (VideoConfig, error) {
	conf := VideoConfig{}
	path := filepath.Join(paths.Config, config)

	for _, ext := range []string{"yaml", "yml", "json"} {
		if _, err := os.Stat(path + "." + ext); err != nil {
			continue
		}

		path = path + "." + ext
		break
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return conf, tracerr.Wrap(err)
	}

	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		return conf, tracerr.Wrap(err)
	}

	conf.Name = config

	if conf.Filter != "" {
		conf.Filter = filepath.Join(paths.Config, conf.Filter)
		conf.Filter, err = filepath.Abs(conf.Filter)
		if err != nil {
			return conf, tracerr.Wrap(err)
		}

		if filepath.Ext(conf.Filter) != ".vpy" {
			return conf, tracerr.New("only .vpy filters are supported")
		}
	}

	return conf, nil
}
