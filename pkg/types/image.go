package types

import (
	"time"
)

type Image struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Tag         string            `json:"tag"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"created_at"`
	Config      ImageConfig       `json:"config"`
	Layers      []string          `json:"layers"`
	Labels      map[string]string `json:"labels"`
}

type ImageConfig struct {
	Env          []string               `json:"env"`
	Cmd          []string               `json:"cmd"`
	Entrypoint   []string               `json:"entrypoint"`
	WorkingDir   string                 `json:"working_dir"`
	ExposedPorts map[string]struct{}    `json:"exposed_ports"`
	Volumes      map[string]struct{}    `json:"volumes"`
	Labels       map[string]string      `json:"labels"`
	StopSignal   string                 `json:"stop_signal"`
}

type ImageFilter struct {
	Name   string
	Labels map[string]string
}

type ImageCreateOptions struct {
	FromImage string `json:"from_image"`
	Tag       string `json:"tag"`
}

type ImageBuildOptions struct {
	Dockerfile  string            `json:"dockerfile"`
	ContextDir  string            `json:"context_dir"`
	Tags        []string          `json:"tags"`
	Labels      map[string]string `json:"labels"`
	NoCache     bool              `json:"no_cache"`
	Remove      bool              `json:"remove"`
	ForceRemove bool              `json:"force_remove"`
}