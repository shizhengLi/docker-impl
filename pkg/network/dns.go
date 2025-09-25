package network

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

type DNSManager struct {
	server      *dns.Server
	records     map[string][]string
	aliases     map[string]string
	containerIP map[string]string
	mu          sync.RWMutex
	listenAddr  string
}

type DNSRecord struct {
	Name  string
	Type  string
	Value string
	TTL   uint32
}

func NewDNSManager(listenAddr string) *DNSManager {
	return &DNSManager{
		server:      &dns.Server{Addr: listenAddr, Net: "udp"},
		records:     make(map[string][]string),
		aliases:     make(map[string]string),
		containerIP: make(map[string]string),
		listenAddr:  listenAddr,
	}
}

func (dm *DNSManager) Start() error {
	// Set up default records
	dm.addDefaultRecords()

	// Configure DNS handler
	dns.HandleFunc(".", dm.handleDNSRequest)

	// Start DNS server
	go func() {
		if err := dm.server.ListenAndServe(); err != nil {
			logrus.Errorf("DNS server error: %v", err)
		}
	}()

	logrus.Infof("DNS server started on %s", dm.listenAddr)
	return nil
}

func (dm *DNSManager) Stop() error {
	if dm.server != nil {
		return dm.server.Shutdown()
	}
	return nil
}

func (dm *DNSManager) addDefaultRecords() {
	// Add default DNS records
	dm.AddRecord("localhost", "A", "127.0.0.1", 3600)
	dm.AddRecord("localhost", "AAAA", "::1", 3600)

	// Add container DNS domain
	dm.AddRecord("mydocker.local", "A", "172.17.0.1", 3600)
}

func (dm *DNSManager) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	for _, q := range r.Question {
		logrus.Debugf("DNS query: %s %s", q.Name, q.Qtype)

		switch q.Qtype {
		case dns.TypeA:
			records := dm.getARecords(q.Name)
			for _, record := range records {
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					A: net.ParseIP(record),
				}
				m.Answer = append(m.Answer, rr)
			}

		case dns.TypeAAAA:
			records := dm.getAAAARecords(q.Name)
			for _, record := range records {
				rr := &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					AAAA: net.ParseIP(record),
				}
				m.Answer = append(m.Answer, rr)
			}

		case dns.TypeCNAME:
			if alias, exists := dm.getAlias(q.Name); exists {
				rr := &dns.CNAME{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeCNAME,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					Target: alias,
				}
				m.Answer = append(m.Answer, rr)
			}

		case dns.TypeTXT:
			// Add TXT records for service discovery
			txtRecord := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"mydocker-container"},
			}
			m.Answer = append(m.Answer, txtRecord)
		}
	}

	w.WriteMsg(m)
}

func (dm *DNSManager) getARecords(name string) []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// Normalize domain name
	name = strings.TrimSuffix(name, ".")

	if records, exists := dm.records[name]; exists {
		return records
	}

	// Try to resolve container name
	if ip, exists := dm.containerIP[name]; exists {
		return []string{ip}
	}

	return []string{}
}

func (dm *DNSManager) getAAAARecords(name string) []string {
	// For now, return empty - IPv6 support can be added later
	return []string{}
}

func (dm *DNSManager) getAlias(name string) (string, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	alias, exists := dm.aliases[name]
	return alias, exists
}

func (dm *DNSManager) AddRecord(name, recordType, value string, ttl uint32) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name = strings.TrimSuffix(name, ".")
	key := fmt.Sprintf("%s:%s", name, recordType)

	if _, exists := dm.records[key]; !exists {
		dm.records[key] = []string{}
	}

	dm.records[key] = append(dm.records[key], value)
	logrus.Debugf("Added DNS record: %s %s -> %s", name, recordType, value)
}

func (dm *DNSManager) RemoveRecord(name, recordType, value string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name = strings.TrimSuffix(name, ".")
	key := fmt.Sprintf("%s:%s", name, recordType)

	if records, exists := dm.records[key]; exists {
		for i, record := range records {
			if record == value {
				dm.records[key] = append(records[:i], records[i+1:]...)
				break
			}
		}

		if len(dm.records[key]) == 0 {
			delete(dm.records, key)
		}
	}

	logrus.Debugf("Removed DNS record: %s %s -> %s", name, recordType, value)
}

func (dm *DNSManager) RegisterContainer(containerID, containerName, ip string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Register container IP
	dm.containerIP[containerName] = ip
	dm.containerIP[containerID] = ip

	// Add A record for container name
	dm.AddRecord(containerName, "A", ip, 300)

	// Add records for service discovery
	serviceName := fmt.Sprintf("%s.mydocker.local", containerName)
	dm.AddRecord(serviceName, "A", ip, 300)

	logrus.Infof("Registered container DNS: %s -> %s", containerName, ip)
}

func (dm *DNSManager) UnregisterContainer(containerID, containerName string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Remove container IP
	if ip, exists := dm.containerIP[containerName]; exists {
		delete(dm.containerIP, containerName)
		delete(dm.containerIP, containerID)

		// Remove DNS records
		dm.RemoveRecord(containerName, "A", ip)

		serviceName := fmt.Sprintf("%s.mydocker.local", containerName)
		dm.RemoveRecord(serviceName, "A", ip)

		logrus.Infof("Unregistered container DNS: %s", containerName)
	}
}

func (dm *DNSManager) AddAlias(name, target string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name = strings.TrimSuffix(name, ".")
	dm.aliases[name] = target

	logrus.Infof("Added DNS alias: %s -> %s", name, target)
}

func (dm *DNSManager) RemoveAlias(name string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name = strings.TrimSuffix(name, ".")
	delete(dm.aliases, name)

	logrus.Infof("Removed DNS alias: %s", name)
}

func (dm *DNSManager) Resolve(name string) ([]string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	name = strings.TrimSuffix(name, ".")

	// Check direct records
	keyA := fmt.Sprintf("%s:A", name)
	if records, exists := dm.records[keyA]; exists {
		return records, nil
	}

	// Check container IP
	if ip, exists := dm.containerIP[name]; exists {
		return []string{ip}, nil
	}

	// Check aliases
	if target, exists := dm.aliases[name]; exists {
		keyTargetA := fmt.Sprintf("%s:A", target)
		if records, exists := dm.records[keyTargetA]; exists {
			return records, nil
		}
	}

	return nil, fmt.Errorf("DNS record not found: %s", name)
}

func (dm *DNSManager) ListRecords() []DNSRecord {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var records []DNSRecord

	for key, values := range dm.records {
		parts := strings.Split(key, ":")
		if len(parts) == 2 {
			for _, value := range values {
				records = append(records, DNSRecord{
					Name:  parts[0],
					Type:  parts[1],
					Value: value,
					TTL:   3600,
				})
			}
		}
	}

	return records
}

func (dm *DNSManager) GetDNSConfig() string {
	return fmt.Sprintf("nameserver %s\nsearch mydocker.local\noptions ndots:0", dm.listenAddr[:strings.Index(dm.listenAddr, ":")])
}

func (dm *DNSManager) CreateResolvConf(containerID string) string {
	return fmt.Sprintf("# Generated by mydocker\nnameserver %s\nsearch mydocker.local\noptions ndots:0 timeout:1 attempts:3", dm.listenAddr[:strings.Index(dm.listenAddr, ":")])
}

type ServiceDiscovery struct {
	dnsManager *DNSManager
	services    map[string]ServiceRecord
	mu          sync.RWMutex
}

type ServiceRecord struct {
	Name      string
	Addresses []string
	Port      int
	Protocol  string
	Metadata  map[string]string
	Timestamp time.Time
}

func NewServiceDiscovery(dnsManager *DNSManager) *ServiceDiscovery {
	return &ServiceDiscovery{
		dnsManager: dnsManager,
		services:   make(map[string]ServiceRecord),
	}
}

func (sd *ServiceDiscovery) RegisterService(serviceName, containerIP string, port int, protocol string, metadata map[string]string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	serviceKey := fmt.Sprintf("%s.%s.%s", serviceName, protocol, port)

	record := ServiceRecord{
		Name:      serviceName,
		Addresses: []string{containerIP},
		Port:      port,
		Protocol:  protocol,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	sd.services[serviceKey] = record

	// Register DNS SRV record
	srvValue := fmt.Sprintf("0 0 %d %s", port, containerIP)
	sd.dnsManager.AddRecord(serviceName, "SRV", srvValue, 300)

	logrus.Infof("Registered service: %s -> %s:%d (%s)", serviceName, containerIP, port, protocol)
}

func (sd *ServiceDiscovery) UnregisterService(serviceName, protocol string, port int) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	serviceKey := fmt.Sprintf("%s.%s.%s", serviceName, protocol, port)

	delete(sd.services, serviceKey)

	// Remove DNS SRV record
	sd.dnsManager.RemoveRecord(serviceName, "SRV", "")

	logrus.Infof("Unregistered service: %s (%s:%d)", serviceName, protocol, port)
}

func (sd *ServiceDiscovery) DiscoverService(serviceName string) ([]ServiceRecord, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	var services []ServiceRecord

	for key, record := range sd.services {
		if strings.HasPrefix(key, serviceName+".") {
			services = append(services, record)
		}
	}

	return services, nil
}

func (sd *ServiceDiscovery) ListServices() []ServiceRecord {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	var services []ServiceRecord
	for _, record := range sd.services {
		services = append(services, record)
	}

	return services
}