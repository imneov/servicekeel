package dns

import (
	"net"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func TestNewServer(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	if s.ipRange != ipRange {
		t.Errorf("NewServer() ipRange = %s; want %s", s.ipRange, ipRange)
	}
}

func TestStartAndStop(t *testing.T) {
	s, err := NewServer("127.0.66.0/24")
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	// Test that Start returns no error on stub implementation
	if err := s.Start("127.0.0.2:5353"); err != nil {
		t.Errorf("Start() returned error: %v", err)
	}
	// Ensure Stop does not panic
	s.Stop()
}

func TestHandleRequest(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	// register default mapping for test
	qname := "test.default.svc."
	wantIP := net.ParseIP("127.0.66.1")
	s.AddMapping(qname, wantIP)
	// Start on random available port
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)
	addr := s.server.PacketConn.LocalAddr().String()
	msg := new(mdns.Msg)
	msg.SetQuestion(qname, mdns.TypeA)
	client := new(mdns.Client)
	resp, _, err := client.Exchange(msg, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("Expected 1 answer; got %d", len(resp.Answer))
	}
	aRec, ok := resp.Answer[0].(*mdns.A)
	if !ok {
		t.Fatalf("Expected A record; got %T", resp.Answer[0])
	}
	if !aRec.A.Equal(wantIP) {
		t.Errorf("Got IP %s; want %s", aRec.A.String(), wantIP.String())
	}
}

func TestGetIP(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	ip := s.GetIP(0)
	if ip != "127.0.66.0" {
		t.Errorf("GetIP() returned %s; want 127.0.66.0", ip)
	}
	ip = s.GetIP(1)
	if ip != "127.0.66.1" {
		t.Errorf("GetIP() returned %s; want 127.0.66.1", ip)
	}
	ip = s.GetIP(255)
	if ip != "127.0.66.255" {
		t.Errorf("GetIP() returned %s; want 127.0.66.255", ip)
	}
	ip = s.GetIP(256)
	if ip != "" {
		t.Errorf("GetIP() returned %s; want empty string", ip)
	}
}

func TestAddMapping(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	// register custom mapping
	qname := "custom.service.svc."
	customIP := net.ParseIP("127.0.66.9")
	s.AddMapping(qname, customIP)

	// start server
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	addr := s.server.PacketConn.LocalAddr().String()
	msg := new(mdns.Msg)
	msg.SetQuestion(qname, mdns.TypeA)
	client := new(mdns.Client)
	resp, _, err := client.Exchange(msg, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("Expected 1 answer; got %d", len(resp.Answer))
	}
	aRec, ok := resp.Answer[0].(*mdns.A)
	if !ok {
		t.Fatalf("Expected A record; got %T", resp.Answer[0])
	}
	if !aRec.A.Equal(customIP) {
		t.Errorf("Mapping failed: got %s; want %s", aRec.A.String(), customIP.String())
	}
}

// TestNoMapping verifies that queries without a mapping return NXDOMAIN.
func TestNoMapping(t *testing.T) {
	s, err := NewServer("127.0.66.0/24")
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)
	addr := s.server.PacketConn.LocalAddr().String()
	msg := new(mdns.Msg)
	qname := "nomap.default.svc."
	msg.SetQuestion(qname, mdns.TypeA)
	client := new(mdns.Client)
	resp, _, err := client.Exchange(msg, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}
	if len(resp.Answer) != 0 {
		t.Fatalf("Expected 0 answers; got %d", len(resp.Answer))
	}
	if resp.Rcode != mdns.RcodeNameError {
		t.Errorf("Expected NXDOMAIN (RcodeNameError); got %d", resp.Rcode)
	}
}
