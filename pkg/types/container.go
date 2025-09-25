package types

import (
	"time"
)

type ContainerStatus string

const (
	StatusCreated   ContainerStatus = "created"
	StatusRunning   ContainerStatus = "running"
	StatusStopped   ContainerStatus = "stopped"
	StatusPaused    ContainerStatus = "paused"
	StatusExited    ContainerStatus = "exited"
	StatusRemoving  ContainerStatus = "removing"
	StatusDead      ContainerStatus = "dead"
)

type Container struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Status        ContainerStatus   `json:"status"`
	PID           int               `json:"pid"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     time.Time         `json:"started_at"`
	FinishedAt    time.Time         `json:"finished_at"`
	Config        ContainerConfig   `json:"config"`
	Network       NetworkSettings   `json:"network_settings"`
	HostConfig    HostConfig        `json:"host_config"`
	Mounts        []Mount           `json:"mounts"`
	Labels        map[string]string `json:"labels"`
	LogPath       string            `json:"log_path"`
	Driver        string            `json:"driver"`
	Platform      string            `json:"platform"`
	RootFS        RootFS            `json:"root_fs"`
}

type ContainerConfig struct {
	Hostname     string                 `json:"hostname"`
	DomainName   string                 `json:"domain_name"`
	User         string                 `json:"user"`
	Env          []string               `json:"env"`
	Cmd          []string               `json:"cmd"`
	Entrypoint   []string               `json:"entrypoint"`
	Image        string                 `json:"image"`
	Labels       map[string]string      `json:"labels"`
	WorkingDir   string                 `json:"working_dir"`
	ExposedPorts map[string]struct{}    `json:"exposed_ports"`
	StopSignal   string                 `json:"stop_signal"`
	Tty          bool                   `json:"tty"`
	OpenStdin    bool                   `json:"open_stdin"`
	StdinOnce    bool                   `json:"stdin_once"`
	AttachStdin  bool                   `json:"attach_stdin"`
	AttachStdout bool                   `json:"attach_stdout"`
	AttachStderr bool                   `json:"attach_stderr"`
}

type HostConfig struct {
	Binds           []string            `json:"binds"`
	PortBindings    map[string][]PortBinding `json:"port_bindings"`
	NetworkMode     string              `json:"network_mode"`
	PublishAllPorts bool                `json:"publish_all_ports"`
	Privileged      bool                `json:"privileged"`
	ReadonlyRootfs  bool                `json:"readonly_rootfs"`
	CPUShares       int64               `json:"cpu_shares"`
	Memory          int64               `json:"memory"`
	MemorySwap      int64               `json:"memory_swap"`
	RestartPolicy   RestartPolicy       `json:"restart_policy"`
	VolumesFrom     []string            `json:"volumes_from"`
}

type RestartPolicy struct {
	Name              string `json:"name"`
	MaximumRetryCount int    `json:"maximum_retry_count"`
}

type PortBinding struct {
	HostIP   string `json:"host_ip"`
	HostPort string `json:"host_port"`
}

type NetworkSettings struct {
	IPAddress   string            `json:"ip_address"`
	Gateway     string            `json:"gateway"`
	Ports       map[string][]PortBinding `json:"ports"`
	NetworkMode string            `json:"network_mode"`
	MacAddress  string            `json:"mac_address"`
	Bridge      string            `json:"bridge"`
	SandboxID   string            `json:"sandbox_id"`
}

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation"`
}

type RootFS struct {
	Type    string   `json:"type"`
	Layers  []string `json:"layers"`
	BaseFS  string   `json:"base_fs"`
}

type ContainerCreateOptions struct {
	Name       string            `json:"name"`
	Config     ContainerConfig   `json:"config"`
	HostConfig HostConfig        `json:"host_config"`
	Labels     map[string]string `json:"labels"`
}

type ContainerListOptions struct {
	All     bool              `json:"all"`
	Limit   int               `json:"limit"`
	Since   string            `json:"since"`
	Before  string            `json:"before"`
	Filters map[string][]string `json:"filters"`
}

type ContainerStartOptions struct {
	DetachKeys string `json:"detach_keys"`
}

type ContainerStopOptions struct {
	Timeout int `json:"timeout"`
}

type ContainerRemoveOptions struct {
	Force      bool `json:"force"`
	RemoveVolumes bool `json:"remove_volumes"`
	RemoveLinks   bool `json:"remove_links"`
}