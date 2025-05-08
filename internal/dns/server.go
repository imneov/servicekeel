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

	for _, q := range req.Question {
		if q.Qtype != mdns.TypeA {
			log.Printf("request ID %d with DNS %s(%s) is not A query, skipping", req.Id, q.Name, q.Qtype)
			continue
		}

		log.Printf("Handling to request ID %d with DNS A query for %s", req.Id, q.Name)

		// 尝试直接匹配
		s.mu.RLock()
		mappedIP, ok := s.mappings[q.Name]
		s.mu.RUnlock()

		// 如果直接匹配失败，尝试别名
		if !ok {
			s.mu.RLock()
			aliasTarget, aliasOk := s.aliases[q.Name]
			if aliasOk {
				mappedIP, ok = s.mappings[aliasTarget]
			}
			s.mu.RUnlock()
		}

		// 如果直接匹配和别名都失败，尝试使用 search 域
		if !ok {
			s.mu.RLock()
			// 获取主机名部分（第一段）
			hostPart := q.Name

			// 尝试每个 search 域
			for _, search := range s.searches {
				fullName := JoinDomain(hostPart, search)
				if ip, exists := s.mappings[fullName]; exists {
					mappedIP = ip
					ok = true
					log.Printf("Found request ID %d match using search domain: %s -> %s", req.Id, q.Name, fullName)
					break
				}
			}
			s.mu.RUnlock()
		}

		if ok {
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
			msg.Rcode = dns.RcodeSuccess // 这行非常关键，确保不是默认的 NXDOMAIN
			break
		} else {
			log.Printf("No DNS to request ID %d with mapping for %s, skipping", req.Id, q.Name)
		}
	}

	// 只有在真的没找到答案时才设置 NXDOMAIN
	if len(msg.Answer) == 0 {
		msg.Rcode = mdns.RcodeNameError
	} else {
		msg.Rcode = mdns.RcodeSuccess // 明确设置成功状态码
	}

	if err := w.WriteMsg(msg); err != nil {
		log.Printf("Failed to request ID %d with write DNS response: %v", req.Id, err)
	}
	log.Printf("Responded to request ID %d with %d answers and code %d", req.Id, len(msg.Answer), msg.Rcode)
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
