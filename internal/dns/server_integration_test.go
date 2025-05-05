package dns

import (
	"net"
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
	expectedIP := net.ParseIP("127.0.66.2")
	s.AddMapping("mysql.", expectedIP)
	s.AddMapping("mysql.default.svc.", expectedIP)
	s.AddMapping("mysql.default.svc.cluster.local.", expectedIP)
	expectedIP = net.ParseIP("127.0.66.4")
	s.AddMapping("mysql.", expectedIP)
	s.AddMapping("mysql.default.svc.", expectedIP)
	s.AddMapping("mysql.default.svc.cluster.local.", expectedIP)
	addr := "127.0.0.1:15353"
	if err := s.Start(addr); err != nil {
		t.Fatalf("Server.Start() failed: %v", err)
	}
	defer s.Stop()

	// 等待服务器启动
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
}
