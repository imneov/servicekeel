package dns

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	mdns "github.com/miekg/dns"
)

// Server is a DNS hijacking server that serves A record responses within a specified IP range.
type Server struct {
	ipRange  string
	ipNet    *net.IPNet
	server   *mdns.Server
	mu       sync.RWMutex
	aliases  map[string]string
	mappings map[string]net.IP
	usedIPs  map[string]struct{}
}

// NewServer creates a new DNS hijacking server for the given IP range.
func NewServer(ipRange string) (*Server, error) {
	// parse IP range to ensure ipNet is ready
	_, ipNet, err := net.ParseCIDR(ipRange)
	if err != nil {
		return nil, fmt.Errorf("parse CIDR %s failed: %w", ipRange, err)
	}
	return &Server{
		ipRange:  ipRange,
		ipNet:    ipNet,
		aliases:  make(map[string]string),
		mappings: make(map[string]net.IP),
		usedIPs:  make(map[string]struct{}),
	}, nil
}

// GetIP 依照ipRange，返回一个ip
func (s *Server) GetIP(idx int) string {
	ip := s.ipNet.IP.Mask(s.ipNet.Mask)
	ip[3] = byte(idx)
	return ip.String()
}

// getUnusedIP 依照ipRange，返回一个未使用的ip
func (s *Server) getUnusedIP() net.IP {
	ip := s.ipNet.IP.Mask(s.ipNet.Mask)
	for i := 0; i < 256; i++ {
		ipStr := ip.String()
		if _, ok := s.usedIPs[ipStr]; !ok {
			s.usedIPs[ipStr] = struct{}{}
			return ip
		}
		ip[3]++
	}
	return nil
}

// AddMapping registers a fixed IP for the given DNS query name.
func (s *Server) AddMapping(name string) (net.IP, error) {
	if ip, ok := s.mappings[name]; ok {
		return ip, nil
	}
	ip := s.getUnusedIP()
	if ip == nil {
		log.Printf("no unused IP found")
		return nil, fmt.Errorf("no unused IP found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usedIPs[ip.String()] = struct{}{}
	s.mappings[name] = ip
	return ip, nil
}

// AddMappingAlias registers a fixed IP for the given DNS query name.
func (s *Server) AddMappingAlias(name string, alias ...string) error {
	for _, a := range alias {
		s.aliases[a] = name
	}
	return nil
}

// RemoveMapping removes a mapping for the given DNS query name.
func (s *Server) RemoveMapping(name string) (net.IP, error) {
	ip, ok := s.mappings[name]
	if !ok {
		log.Printf("no mapping for %s", name)
		return nil, fmt.Errorf("no mapping for %s", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mappings, name)
	delete(s.usedIPs, ip.String())
	for k, v := range s.aliases {
		if v == name {
			delete(s.aliases, k)
		}
	}
	return ip, nil
}

// Start begins listening for DNS queries on the given UDP address.
// It binds the UDP socket first to catch errors, logs the binding, then serves in the background.
func (s *Server) Start(address string) error {

	// create serve mux
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handleRequest)

	// attempt to bind to given address
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Printf("bind UDP %s failed: %v, falling back to wildcard", address, err)
		// fallback to wildcard on same port
		_, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return fmt.Errorf("invalid address %s: %w", address, splitErr)
		}
		conn, err = net.ListenPacket("udp", ":"+port)
		if err != nil {
			return fmt.Errorf("bind UDP wildcard :%s failed: %w", port, err)
		}
		log.Printf("DNS server bound to wildcard :%s", port)
	} else {
		log.Printf("DNS server bound to %s", conn.LocalAddr().String())
	}
	// initialize and run DNS server
	srv := &mdns.Server{PacketConn: conn, Handler: mux}
	s.server = srv
	go func() {
		if err := srv.ActivateAndServe(); err != nil {
			log.Printf("DNS server error: %v", err)
		}
	}()
	return nil
}

// handleRequest processes incoming DNS queries and returns A records based on mappings.
func (s *Server) handleRequest(w mdns.ResponseWriter, req *mdns.Msg) {
	// log incoming query
	log.Printf("DNS request received: %v from %v", req.Question, w.RemoteAddr())
	msg := new(mdns.Msg)
	msg.SetReply(req)
	for _, q := range req.Question {
		if q.Qtype != mdns.TypeA {
			continue
		}
		log.Printf("Handling DNS A query for %s", q.Name)
		s.mu.RLock()
		mappedIP, ok := s.mappings[q.Name]
		if !ok {
			mappedIP, ok = s.mappings[s.aliases[q.Name]]
		}
		s.mu.RUnlock()
		if !ok {
			if idx := strings.Index(q.Name, "."); idx > 0 {
				bare := q.Name[:idx+1]
				s.mu.RLock()
				mappedIP, ok = s.mappings[bare]
				s.mu.RUnlock()
			}
		}
		if !ok {
			log.Printf("No DNS mapping for %s, skipping", q.Name)
			continue
		}
		rr := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   q.Name,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
				Ttl:    5,
			},
			A: mappedIP,
		}
		msg.Answer = append(msg.Answer, rr)
		log.Printf("Mapped DNS %s to IP %s", q.Name, mappedIP)
	}
	if len(msg.Answer) == 0 {
		// no mapping found: return NXDOMAIN
		msg.Rcode = mdns.RcodeNameError
	}
	if err := w.WriteMsg(msg); err != nil {
		log.Printf("Failed to write DNS response: %v", err)
	}
	log.Printf("Responded with %d answers", len(msg.Answer))
}

// Stop stops the DNS server and cleans up resources.
func (s *Server) Stop() {
	if s.server != nil {
		_ = s.server.Shutdown()
	}
}

// ServeDNS handles DNS queries and hijacks service names to local ipRange.
func (s *Server) ServeDNS(w mdns.ResponseWriter, req *mdns.Msg) {
	// stub implementation: reply with same question and no answers
	msg := new(mdns.Msg)
	msg.SetReply(req)
	w.WriteMsg(msg)
}
