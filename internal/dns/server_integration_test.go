package dns

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNSHijackIntegration(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}
	// register default mapping for integration test
	expectedIP, err := s.AddMapping("mysql.")
	if err != nil {
		t.Fatalf("AddMapping() returned error: %v", err)
	}
	s.AddMappingAlias("mysql.", "mysql.default.svc.", "mysql.default.svc.cluster.local.")
	addr := "127.0.0.1:15353"
	if err := s.Start(addr); err != nil {
		t.Fatalf("Server.Start() failed: %v", err)
	}
	defer s.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	client := new(dns.Client)
	for _, fqdn := range []string{"mysql.default.svc.", "mysql.default.svc.cluster.local.", "mysql."} {
		msg := new(dns.Msg)
		msg.SetQuestion(fqdn, dns.TypeA)

		resp, _, err := client.Exchange(msg, addr)
		if err != nil {
			t.Fatalf("DNS query Exchange() failed: %v", err)
		}
		if len(resp.Answer) != 1 {
			t.Fatalf("Expected 1 DNS answer, got %d", len(resp.Answer))
		}
		aRec, ok := resp.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("Answer is not A record: %T", resp.Answer[0])
		}
		if !aRec.A.Equal(expectedIP) {
			t.Errorf("Expected IP %v, got %v", expectedIP, aRec.A)
		}
	}
	// Test mapping removal
	s.RemoveMapping("mysql.")
	msg := new(dns.Msg)
	msg.SetQuestion("mysql.", dns.TypeA)
	resp, _, err := client.Exchange(msg, addr)
	if err != nil {
		t.Fatalf("DNS query Exchange() failed: %v", err)
	}
	if len(resp.Answer) != 0 {
		t.Fatalf("Expected 0 DNS answer, got %d", len(resp.Answer))
	}
	client = new(dns.Client)
	for _, fqdn := range []string{"mysql.default.svc.", "mysql.default.svc.cluster.local.", "mysql."} {
		msg := new(dns.Msg)
		msg.SetQuestion(fqdn, dns.TypeA)

		resp, _, err := client.Exchange(msg, addr)
		if err != nil {
			t.Fatalf("DNS query Exchange() failed: %v", err)
		}
		if len(resp.Answer) != 0 {
			t.Fatalf("Expected 0 DNS answer, got %d", len(resp.Answer))
		}
	}
}
