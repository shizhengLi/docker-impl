package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type BridgeManager struct {
	bridgeName string
	subnet     *net.IPNet
	gateway    net.IP
	usedIPs    map[string]bool
	mu         sync.RWMutex
}

func NewBridgeManager() (*BridgeManager, error) {
	defaultSubnet := "172.17.0.0/16"
	_, ipNet, err := net.ParseCIDR(defaultSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subnet: %v", err)
	}

	gateway := net.ParseIP("172.17.0.1")
	if gateway == nil {
		return nil, fmt.Errorf("failed to parse gateway IP")
	}

	bm := &BridgeManager{
		bridgeName: "mydocker0",
		subnet:     ipNet,
		gateway:    gateway,
		usedIPs:    make(map[string]bool),
	}

	// Reserve gateway IP
	bm.usedIPs[bm.gateway.String()] = true

	if err := bm.createBridge(); err != nil {
		return nil, fmt.Errorf("failed to create bridge: %v", err)
	}

	return bm, nil
}

func (bm *BridgeManager) createBridge() error {
	// Check if bridge already exists
	if bm.bridgeExists() {
		logrus.Infof("Bridge %s already exists", bm.bridgeName)
		return nil
	}

	// Create bridge
	cmd := exec.Command("ip", "link", "add", "name", bm.bridgeName, "type", "bridge")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create bridge: %v", err)
	}

	// Bring bridge up
	cmd = exec.Command("ip", "link", "set", "dev", bm.bridgeName, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring bridge up: %v", err)
	}

	// Assign IP to bridge
	cmd = exec.Command("ip", "addr", "add", bm.gateway.String()+"/16", "dev", bm.bridgeName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to assign IP to bridge: %v", err)
	}

	// Enable IP forwarding
	if err := bm.enableIPForwarding(); err != nil {
		logrus.Warnf("Failed to enable IP forwarding: %v", err)
	}

	// Configure iptables for NAT
	if err := bm.configureIptables(); err != nil {
		logrus.Warnf("Failed to configure iptables: %v", err)
	}

	logrus.Infof("Bridge %s created successfully", bm.bridgeName)
	return nil
}

func (bm *BridgeManager) bridgeExists() bool {
	cmd := exec.Command("ip", "link", "show", bm.bridgeName)
	return cmd.Run() == nil
}

func (bm *BridgeManager) enableIPForwarding() error {
	// Enable IPv4 forwarding
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func (bm *BridgeManager) configureIptables() error {
	// Add NAT rule for outbound traffic
	cmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", bm.subnet.String(), "!", "-o", bm.bridgeName, "-j", "MASQUERADE")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add NAT rule: %v", err)
	}

	// Add forwarding rules
	cmd = exec.Command("iptables", "-A", "FORWARD", "-i", bm.bridgeName, "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add forward rule: %v", err)
	}

	cmd = exec.Command("iptables", "-A", "FORWARD", "-o", bm.bridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add forward rule: %v", err)
	}

	return nil
}

func (bm *BridgeManager) AllocateIP() (net.IP, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Find available IP in subnet
	for ip := bm.nextIP(bm.gateway); bm.subnet.Contains(ip); ip = bm.nextIP(ip) {
		ipStr := ip.String()
		if !bm.usedIPs[ipStr] {
			bm.usedIPs[ipStr] = true
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no available IP in subnet")
}

func (bm *BridgeManager) nextIP(ip net.IP) net.IP {
	nextIP := make(net.IP, len(ip))
	copy(nextIP, ip)

	for j := len(nextIP) - 1; j >= 0; j-- {
		nextIP[j]++
		if nextIP[j] > 0 {
			break
		}
	}

	return nextIP
}

func (bm *BridgeManager) ReleaseIP(ip net.IP) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	ipStr := ip.String()
	delete(bm.usedIPs, ipStr)
	logrus.Debugf("Released IP: %s", ipStr)
}

func (bm *BridgeManager) CreateVethPair(containerID string) (string, string, error) {
	vethHost := "veth" + containerID[:8] + "h"
	vethContainer := "veth" + containerID[:8] + "c"

	// Create veth pair
	cmd := exec.Command("ip", "link", "add", vethHost, "type", "veth", "peer", "name", vethContainer)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to create veth pair: %v", err)
	}

	// Connect host end to bridge
	cmd = exec.Command("ip", "link", "set", vethHost, "master", bm.bridgeName)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to connect veth to bridge: %v", err)
	}

	// Bring host end up
	cmd = exec.Command("ip", "link", "set", "dev", vethHost, "up")
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to bring veth host up: %v", err)
	}

	logrus.Infof("Created veth pair: %s <-> %s", vethHost, vethContainer)
	return vethHost, vethContainer, nil
}

func (bm *BridgeManager) ConfigureContainerNetwork(containerID, vethContainer string, containerIP net.IP) error {
	// Move veth to container network namespace
	// This would typically be done when the container is created
	// For now, we'll just prepare the veth interface

	// Bring container veth up
	cmd := exec.Command("ip", "link", "set", "dev", vethContainer, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring veth container up: %v", err)
	}

	logrus.Infof("Configured container network: %s -> %s", containerID, containerIP)
	return nil
}

func (bm *BridgeManager) SetupPortMapping(containerID string, portMappings []PortMapping) error {
	for _, mapping := range portMappings {
		if err := bm.addPortMapping(containerID, mapping); err != nil {
			logrus.Errorf("Failed to add port mapping %v: %v", mapping, err)
			continue
		}
	}
	return nil
}

func (bm *BridgeManager) addPortMapping(containerID string, mapping PortMapping) error {
	// Add iptables rule for port mapping
	rule := fmt.Sprintf("-t nat -A PREROUTING -p %s --dport %d -j DNAT --to-destination %s:%d",
		mapping.Protocol, mapping.HostPort, mapping.ContainerIP, mapping.ContainerPort)

	cmd := exec.Command("iptables", strings.Fields(rule)...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add port mapping rule: %v", err)
	}

	logrus.Infof("Added port mapping: %s:%d -> %s:%d",
		"0.0.0.0", mapping.HostPort, mapping.ContainerIP, mapping.ContainerPort)
	return nil
}

func (bm *BridgeManager) RemovePortMapping(containerID string, portMappings []PortMapping) {
	for _, mapping := range portMappings {
		bm.removePortMapping(containerID, mapping)
	}
}

func (bm *BridgeManager) removePortMapping(containerID string, mapping PortMapping) {
	rule := fmt.Sprintf("-t nat -D PREROUTING -p %s --dport %d -j DNAT --to-destination %s:%d",
		mapping.Protocol, mapping.HostPort, mapping.ContainerIP, mapping.ContainerPort)

	cmd := exec.Command("iptables", strings.Fields(rule)...)
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to remove port mapping %v: %v", mapping, err)
	}
}

func (bm *BridgeManager) GetBridgeInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":    bm.bridgeName,
		"subnet":  bm.subnet.String(),
		"gateway": bm.gateway.String(),
		"used_ips": len(bm.usedIPs),
	}
}

func (bm *BridgeManager) Cleanup() {
	// Remove bridge if it exists
	if bm.bridgeExists() {
		cmd := exec.Command("ip", "link", "del", bm.bridgeName)
		if err := cmd.Run(); err != nil {
			logrus.Warnf("Failed to remove bridge: %v", err)
		}
	}

	// Clean up iptables rules
	bm.cleanupIptables()
}

func (bm *BridgeManager) cleanupIptables() {
	// This is a simplified cleanup - in practice, you'd want to remove specific rules
	// rather than flushing entire chains
	cmd := exec.Command("iptables", "-t", "nat", "-F")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to flush iptables nat table: %v", err)
	}

	cmd = exec.Command("iptables", "-F")
	if err := cmd.Run(); err != nil {
		logrus.Warnf("Failed to flush iptables filter table: %v", err)
	}
}

func (bm *BridgeManager) GetContainerNetworkStats(containerID string) map[string]interface{} {
	// This would collect network statistics for the container
	// For now, return basic information
	return map[string]interface{}{
		"container_id": containerID,
		"bridge":       bm.bridgeName,
		"network_mode": "bridge",
	}
}