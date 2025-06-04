package dns

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
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
	searches []string
}

func init() {
	// 设置日志格式，显示微秒级时间戳
	log.SetFlags(log.Lmicroseconds | log.LstdFlags)
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

// GetIP returns an IP address based on the index within the ipRange
func (s *Server) GetIP(idx int) string {
	ip := s.ipNet.IP.Mask(s.ipNet.Mask)
	ip[3] = byte(idx)
	return ip.String()
}

// getUnusedIP returns an unused IP address from the ipRange
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
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
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
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	for _, a := range alias {
		if !strings.HasSuffix(a, ".") {
			a = a + "."
		}
		s.aliases[a] = name
	}
	return nil
}

// RemoveMapping removes a mapping for the given DNS query name.
func (s *Server) RemoveMapping(name string) (net.IP, error) {
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
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

// SetSearchDomains sets the search domains for DNS resolution
func (s *Server) SetSearchDomains(domains []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searches = domains
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
	log.Printf("handleRequest %d", req.Id)
	msg := new(mdns.Msg)
	msg.SetReply(req)

	// Process each question in the request
	for _, q := range req.Question {
		switch q.Qtype {
		case mdns.TypeAAAA:
			// Handle AAAA queries with success and no records (continues to next question)
			log.Printf("Request ID %d with AAAA query for %s", req.Id, q.Name)
			continue

		case mdns.TypeA:
			// Process A record queries
			log.Printf("Handling request ID %d with DNS A query for %s", req.Id, q.Name)

			// Try to resolve the query to an IP address
			mappedIP, ok := s.resolveQuery(q.Name, req.Id)

			if ok {
				// Add answer if successful resolution
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
				log.Printf("Mapped request ID %d DNS %s to IP %s", req.Id, q.Name, mappedIP)
				msg.Authoritative = true
				msg.Rcode = dns.RcodeSuccess
				break
			} else {
				log.Printf("No mapping for request ID %d with DNS %s, skipping", req.Id, q.Name)
			}

		default:
			// Skip other query types
			log.Printf("Request ID %d with DNS %s(%d) is not A query, skipping", req.Id, q.Name, q.Qtype)
		}
	}

	// Set appropriate response code
	if len(msg.Answer) == 0 {
		// No answers found - set response code based on query type
		if hasQueryType(req, mdns.TypeAAAA) && !hasQueryType(req, mdns.TypeA) {
			// Pure AAAA query - return success with empty answer
			log.Printf("Request ID %d with AAAA query for %s, setting Rcode to Success", req.Id, req.Question[0].Name)
			msg.Rcode = mdns.RcodeSuccess
		} else {
			// A query with no answer - return NXDOMAIN
			log.Printf("Request ID %d with A query for %s, setting Rcode to NameError", req.Id, req.Question[0].Name)
			msg.Rcode = mdns.RcodeNameError
		}
	} else {
		msg.Rcode = mdns.RcodeSuccess
	}

	// Send the response
	if err := w.WriteMsg(msg); err != nil {
		log.Printf("Failed to write DNS response for request ID %d: %v", req.Id, err)
	}
	log.Printf("Responded to request ID %d with %d answers and code %d", req.Id, len(msg.Answer), msg.Rcode)
}

// resolveQuery attempts to resolve a DNS query name to an IP address using direct mappings,
// aliases, and search domains. Returns the IP and success status.
func (s *Server) resolveQuery(queryName string, reqID uint16) (net.IP, bool) {
	// Try direct mapping first
	s.mu.RLock()
	mappedIP, ok := s.mappings[queryName]
	s.mu.RUnlock()
	if ok {
		return mappedIP, true
	}

	// Try alias resolution
	s.mu.RLock()
	aliasTarget, aliasOk := s.aliases[queryName]
	if aliasOk {
		mappedIP, ok = s.mappings[aliasTarget]
	}
	s.mu.RUnlock()
	if ok {
		return mappedIP, true
	}

	// Try search domains
	s.mu.RLock()
	for _, search := range s.searches {
		fullName := JoinDomain(queryName, search)
		if ip, exists := s.mappings[fullName]; exists {
			log.Printf("Found request ID %d match using search domain: %s -> %s", reqID, queryName, fullName)
			s.mu.RUnlock()
			return ip, true
		}
	}
	s.mu.RUnlock()

	return nil, false
}

// hasQueryType checks if a DNS message contains a question of the specified type
func hasQueryType(req *mdns.Msg, qtype uint16) bool {
	for _, q := range req.Question {
		if q.Qtype == qtype {
			return true
		}
	}
	return false
}

// Stop stops the DNS server and cleans up resources.
func (s *Server) Stop() {
	if s.server != nil {
		_ = s.server.Shutdown()
	}
}

// ServeDNS handles DNS queries and hijacks service names to local ipRange.
// func (s *Server) ServeDNS(w mdns.ResponseWriter, req *mdns.Msg) {
// 	// stub implementation: reply with same question and no answers
// 	msg := new(mdns.Msg)
// 	msg.SetReply(req)
// 	w.WriteMsg(msg)
// }

func JoinDomain(domain, search string) string {
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}
	if !strings.HasSuffix(search, ".") {
		search = search + "."
	}
	return domain + search
}
