package network

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type NetworkMode string

const (
	NetworkModeBridge NetworkMode = "bridge"
	NetworkModeHost   NetworkMode = "host"
	NetworkModeNone   NetworkMode = "none"
	NetworkModeCustom NetworkMode = "custom"
)

type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	ContainerIP   string `json:"container_ip"`
	HostIP        string `json:"host_ip"`
}

type NetworkConfig struct {
	Mode          NetworkMode   `json:"mode"`
	IPAddress     string        `json:"ip_address"`
	MacAddress    string        `json:"mac_address"`
	PortMappings  []PortMapping `json:"port_mappings"`
	DNS           []string      `json:"dns"`
	NetworkName   string        `json:"network_name"`
	Aliases       []string      `json:"aliases"`
	Hostname      string        `json:"hostname"`
	DomainName    string        `json:"domain_name"`
}

type NetworkSettings struct {
	IPAddress   string            `json:"ip_address"`
	Gateway     string            `json:"gateway"`
	MacAddress  string            `json:"mac_address"`
	Ports       map[string][]PortBinding `json:"ports"`
	NetworkMode string            `json:"network_mode"`
	DNS         []string          `json:"dns"`
	NetworkID   string            `json:"network_id"`
	EndpointID  string            `json:"endpoint_id"`
	SandboxID   string            `json:"sandbox_id"`
}

type PortBinding struct {
	HostIP   string `json:"host_ip"`
	HostPort string `json:"host_port"`
}

type Manager struct {
	bridgeManager *BridgeManager
	dnsManager    *DNSManager
	serviceDisc   *ServiceDiscovery
	networks      map[string]*NetworkConfig
	containerNet map[string]*NetworkSettings
	mu            sync.RWMutex
	config        *NetworkConfig
}

type Network struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Driver   string          `json:"driver"`
	Scope    string          `json:"scope"`
	Subnet   string          `json:"subnet"`
	Gateway  string          `json:"gateway"`
	Created  string          `json:"created"`
	Options  map[string]interface{} `json:"options"`
	IPAM     IPAM            `json:"ipam"`
}

type IPAM struct {
	Driver  string   `json:"driver"`
	Options map[string]interface{} `json:"options"`
	Config  []IPAMConfig `json:"config"`
}

type IPAMConfig struct {
	Subnet string `json:"subnet"`
	IPRange string `json:"ip_range"`
	Gateway string `json:"gateway"`
}

var (
	networkManager *Manager
	managerOnce    sync.Once
)

func GetNetworkManager() *Manager {
	managerOnce.Do(func() {
		config := &NetworkConfig{
			Mode: NetworkModeBridge,
		}
		networkManager = NewManager(config)
	})
	return networkManager
}

func NewManager(config *NetworkConfig) *Manager {
	m := &Manager{
		config:       config,
		networks:     make(map[string]*NetworkConfig),
		containerNet: make(map[string]*NetworkSettings),
	}

	// Initialize bridge manager
	if config.Mode == NetworkModeBridge {
		bridgeMgr, err := NewBridgeManager()
		if err != nil {
			logrus.Errorf("Failed to create bridge manager: %v", err)
		} else {
			m.bridgeManager = bridgeMgr
		}
	}

	// Initialize DNS manager
	m.dnsManager = NewDNSManager("172.17.0.1:53")
	if err := m.dnsManager.Start(); err != nil {
		logrus.Errorf("Failed to start DNS manager: %v", err)
	}

	// Initialize service discovery
	m.serviceDisc = NewServiceDiscovery(m.dnsManager)

	// Create default bridge network
	m.createDefaultNetwork()

	logrus.Info("Network manager initialized")
	return m
}

func (m *Manager) createDefaultNetwork() {
	defaultNetwork := &Network{
		ID:      "mydocker0",
		Name:    "bridge",
		Driver:  "bridge",
		Scope:   "local",
		Subnet:  "172.17.0.0/16",
		Gateway: "172.17.0.1",
		Created: "now",
		Options: map[string]interface{}{
			"com.docker.network.bridge.default_bridge": "true",
			"com.docker.network.bridge.enable_icc":     "true",
			"com.docker.network.bridge.name":          "mydocker0",
		},
		IPAM: IPAM{
			Driver: "default",
			Config: []IPAMConfig{
				{
					Subnet:  "172.17.0.0/16",
					Gateway: "172.17.0.1",
				},
			},
		},
	}

	// Store network configuration
	m.networks["bridge"] = &NetworkConfig{
		Mode:        NetworkModeBridge,
		NetworkName: "bridge",
	}

	logrus.Info("Default bridge network created")
}

func (m *Manager) CreateContainerNetwork(containerID, containerName string, config *NetworkConfig) (*NetworkSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logrus.Infof("Creating network for container %s", containerID)

	settings := &NetworkSettings{
		NetworkMode: string(config.Mode),
		DNS:         config.DNS,
		NetworkID:   "mydocker0",
		SandboxID:   containerID,
	}

	switch config.Mode {
	case NetworkModeBridge:
		return m.setupBridgeNetwork(containerID, containerName, config, settings)
	case NetworkModeHost:
		return m.setupHostNetwork(settings)
	case NetworkModeNone:
		return m.setupNoneNetwork(settings)
	default:
		return nil, fmt.Errorf("unsupported network mode: %s", config.Mode)
	}
}

func (m *Manager) setupBridgeNetwork(containerID, containerName string, config *NetworkConfig, settings *NetworkSettings) (*NetworkSettings, error) {
	if m.bridgeManager == nil {
		return nil, fmt.Errorf("bridge manager not available")
	}

	// Allocate IP for container
	containerIP, err := m.bridgeManager.AllocateIP()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %v", err)
	}

	// Create veth pair
	vethHost, vethContainer, err := m.bridgeManager.CreateVethPair(containerID)
	if err != nil {
		m.bridgeManager.ReleaseIP(containerIP)
		return nil, fmt.Errorf("failed to create veth pair: %v", err)
	}

	// Configure container network
	err = m.bridgeManager.ConfigureContainerNetwork(containerID, vethContainer, containerIP)
	if err != nil {
		m.bridgeManager.ReleaseIP(containerIP)
		return nil, fmt.Errorf("failed to configure container network: %v", err)
	}

	// Setup port mappings
	if len(config.PortMappings) > 0 {
		settings.Ports = make(map[string][]PortBinding)
		for _, mapping := range config.PortMappings {
			// Update mapping with container IP
			mapping.ContainerIP = containerIP.String()

			// Add port mapping to bridge
			err = m.bridgeManager.SetupPortMapping(containerID, []PortMapping{mapping})
			if err != nil {
				logrus.Warnf("Failed to setup port mapping %v: %v", mapping, err)
				continue
			}

			// Add to settings
			portKey := fmt.Sprintf("%d/%s", mapping.ContainerPort, mapping.Protocol)
			settings.Ports[portKey] = []PortBinding{
				{
					HostIP:   mapping.HostIP,
					HostPort: fmt.Sprintf("%d", mapping.HostPort),
				},
			}
		}
	}

	// Set network settings
	settings.IPAddress = containerIP.String()
	settings.Gateway = m.bridgeManager.gateway.String()
	settings.EndpointID = vethHost[:12] // Use first 12 chars as endpoint ID

	// Register container DNS
	m.dnsManager.RegisterContainer(containerID, containerName, containerIP.String())

	// Register aliases
	for _, alias := range config.Aliases {
		m.dnsManager.AddAlias(alias, containerName)
	}

	// Store network settings
	m.containerNet[containerID] = settings

	logrus.Infof("Bridge network created for container %s: %s", containerID, containerIP)
	return settings, nil
}

func (m *Manager) setupHostNetwork(settings *NetworkSettings) (*NetworkSettings, error) {
	// For host network, container uses host's network stack
	settings.NetworkMode = "host"
	settings.IPAddress = "127.0.0.1" // This would be host's IP in reality

	logrus.Infof("Host network created for container")
	return settings, nil
}

func (m *Manager) setupNoneNetwork(settings *NetworkSettings) (*NetworkSettings, error) {
	// For none network, container has no network access
	settings.NetworkMode = "none"
	settings.IPAddress = ""

	logrus.Infof("None network created for container")
	return settings, nil
}

func (m *Manager) RemoveContainerNetwork(containerID, containerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logrus.Infof("Removing network for container %s", containerID)

	settings, exists := m.containerNet[containerID]
	if !exists {
		return fmt.Errorf("network settings not found for container %s", containerID)
	}

	// Unregister DNS
	m.dnsManager.UnregisterContainer(containerID, containerName)

	// Remove port mappings
	if m.bridgeManager != nil {
		m.bridgeManager.RemovePortMapping(containerID, nil)
	}

	// Release IP if using bridge network
	if settings.NetworkMode == "bridge" && m.bridgeManager != nil {
		if settings.IPAddress != "" {
			ip := net.ParseIP(settings.IPAddress)
			if ip != nil {
				m.bridgeManager.ReleaseIP(ip)
			}
		}
	}

	// Remove network settings
	delete(m.containerNet, containerID)

	logrus.Infof("Network removed for container %s", containerID)
	return nil
}

func (m *Manager) GetContainerNetwork(containerID string) (*NetworkSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	settings, exists := m.containerNet[containerID]
	if !exists {
		return nil, fmt.Errorf("network settings not found for container %s", containerID)
	}

	return settings, nil
}

func (m *Manager) ListNetworks() []Network {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var networks []Network

	// Add default bridge network
	bridgeNetwork := Network{
		ID:      "mydocker0",
		Name:    "bridge",
		Driver:  "bridge",
		Scope:   "local",
		Subnet:  "172.17.0.0/16",
		Gateway: "172.17.0.1",
		Created: "now",
		Options: map[string]interface{}{
			"com.docker.network.bridge.default_bridge": "true",
			"com.docker.network.bridge.enable_icc":     "true",
		},
	}

	networks = append(networks, bridgeNetwork)

	return networks
}

func (m *Manager) GetNetworkStats(containerID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	settings, exists := m.containerNet[containerID]
	if !exists {
		return nil, fmt.Errorf("network settings not found for container %s", containerID)
	}

	stats := map[string]interface{}{
		"container_id": containerID,
		"network_mode": settings.NetworkMode,
		"ip_address":   settings.IPAddress,
		"gateway":      settings.Gateway,
	}

	if m.bridgeManager != nil {
		bridgeStats := m.bridgeManager.GetContainerNetworkStats(containerID)
		for k, v := range bridgeStats {
			stats[k] = v
		}
	}

	return stats, nil
}

func (m *Manager) GetDNSConfig(containerID string) string {
	return m.dnsManager.GetDNSConfig()
}

func (m *Manager) CreateResolvConf(containerID string) string {
	return m.dnsManager.CreateResolvConf(containerID)
}

func (m *Manager) RegisterService(serviceName, containerID string, port int, protocol string, metadata map[string]string) error {
	m.mu.RLock()
	settings, exists := m.containerNet[containerID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("container %s not found", containerID)
	}

	if settings.IPAddress == "" {
		return fmt.Errorf("container %s has no IP address", containerID)
	}

	m.serviceDisc.RegisterService(serviceName, settings.IPAddress, port, protocol, metadata)
	return nil
}

func (m *Manager) DiscoverService(serviceName string) ([]ServiceRecord, error) {
	return m.serviceDisc.DiscoverService(serviceName)
}

func (m *Manager) ListServices() []ServiceRecord {
	return m.serviceDisc.ListServices()
}

func (m *Manager) Cleanup() {
	if m.bridgeManager != nil {
		m.bridgeManager.Cleanup()
	}

	if m.dnsManager != nil {
		m.dnsManager.Stop()
	}

	logrus.Info("Network manager cleaned up")
}