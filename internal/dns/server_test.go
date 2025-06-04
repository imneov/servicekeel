package dns

import (
	"fmt"
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
	wantIP, err := s.AddMapping(qname)
	if err != nil {
		t.Fatalf("AddMapping() returned error: %v", err)
	}
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
	// test AAAA query
	msg.SetQuestion(qname, mdns.TypeAAAA)
	resp, _, err = client.Exchange(msg, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}
	if len(resp.Answer) != 0 {
		t.Fatalf("Expected 0 answer; got %d", len(resp.Answer))
	}
	if resp.Rcode != mdns.RcodeSuccess {
		t.Errorf("Expected RcodeSuccess; got %d", resp.Rcode)
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
	s.AddMapping(qname)

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

func TestAddMappingAndRemoveMapping(t *testing.T) {
	domainTmpl := "mysql-%d.ns"
	IPtmpl := "127.0.66.%d"
	s, err := NewServer("127.0.66.0/24")
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}

	for i := 0; i < 10; i++ {
		s.AddMapping(fmt.Sprintf(domainTmpl, i))
	}

	// start server
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	addr := s.server.PacketConn.LocalAddr().String()
	for i := 0; i < 10; i++ {
		msg := new(mdns.Msg)
		qname := fmt.Sprintf(domainTmpl, i)
		msg.SetQuestion(qname+".", mdns.TypeA)
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
		IP := fmt.Sprintf(IPtmpl, i)
		customIP := net.ParseIP(IP)
		if !aRec.A.Equal(customIP) {
			t.Errorf("Mapping failed: got %s; (%s)want %s", aRec.A.String(), IP, customIP.String())
		}
	}

	for i := 0; i < 10; i = i + 2 {
		// remove "127.0.66.0/2/4/6/8"
		s.RemoveMapping(fmt.Sprintf(domainTmpl, i))
	}

	qname := "redis-127.0.66.0"
	s.AddMapping(qname) // 127.0.66.0
	qname = "redis-127.0.66.2"
	s.AddMapping(qname) // 127.0.66.2
	qname = "redis-127.0.66.4"
	s.AddMapping(qname) // 127.0.66.4
	qname = "redis-127.0.66.6"
	s.AddMapping(qname) // 127.0.66.6
	msg := new(mdns.Msg)
	msg.SetQuestion(qname+".", mdns.TypeA)
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
	IP := "127.0.66.2"
	customIP := net.ParseIP(IP)
	if !aRec.A.Equal(customIP) {
		t.Errorf("Mapping failed: got %s; (%s)want %s", aRec.A.String(), IP, customIP.String())
	}

}

func TestSearchDomains(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}

	// 设置 search 域
	searchDomains := []string{
		"default.svc.cluster-a.local",
		"svc.cluster-a.local",
		"cluster-a.local",
	}
	s.SetSearchDomains(searchDomains)

	// 注册一个完整的域名映射
	fullName := "test.default.svc.cluster-a.local."
	wantIP, err := s.AddMapping(fullName)
	if err != nil {
		t.Fatalf("AddMapping() returned error: %v", err)
	}

	// 启动服务器
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	addr := s.server.PacketConn.LocalAddr().String()
	client := new(mdns.Client)

	// 测试用例：使用不同的域名格式查询
	testCases := []struct {
		name     string
		query    string
		expected bool
	}{
		// {"full name", fullName, true},
		{"short name", "test.", true},
		{"partial name", "test.default.", true},
		{"non-existent", "nonexistent.", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := new(mdns.Msg)
			msg.SetQuestion(tc.query, mdns.TypeA)
			resp, _, err := client.Exchange(msg, addr)
			if err != nil {
				t.Fatalf("DNS query failed: %v", err)
			}

			if tc.expected {
				if len(resp.Answer) != 1 {
					t.Fatalf("Expected 1 answer %s; got %d", tc.query, len(resp.Answer))
				}
				aRec, ok := resp.Answer[0].(*mdns.A)
				if !ok {
					t.Fatalf("Expected A record; got %T", resp.Answer[0])
				}
				if !aRec.A.Equal(wantIP) {
					t.Errorf("Got IP %s; want %s", aRec.A.String(), wantIP.String())
				}
			} else {
				if len(resp.Answer) != 0 {
					t.Fatalf("Expected 0 answers; got %d", len(resp.Answer))
				}
				if resp.Rcode != mdns.RcodeNameError {
					t.Errorf("Expected NXDOMAIN (RcodeNameError); got %d", resp.Rcode)
				}
			}
		})
	}
}

func TestSearchDomainsOrder(t *testing.T) {
	ipRange := "127.0.66.0/24"
	s, err := NewServer(ipRange)
	if err != nil {
		t.Fatalf("NewServer() returned error: %v", err)
	}

	// 设置 search 域
	searchDomains := []string{
		"default.svc.cluster-a.local",
		"svc.cluster-a.local",
		"cluster-a.local",
	}
	s.SetSearchDomains(searchDomains)

	// 注册多个不同后缀的域名映射
	domains := map[string]net.IP{
		"test.default.svc.cluster-a.local.": net.ParseIP("127.0.66.1"),
		"test.svc.cluster-a.local.":         net.ParseIP("127.0.66.2"),
		"test.cluster-a.local.":             net.ParseIP("127.0.66.3"),
	}

	for domain, ip := range domains {
		s.mappings[domain] = ip
	}

	// 启动服务器
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	addr := s.server.PacketConn.LocalAddr().String()
	client := new(mdns.Client)

	// 测试短域名查询应该匹配第一个有效的 search 域
	msg := new(mdns.Msg)
	msg.SetQuestion("test.", mdns.TypeA)
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
	if !aRec.A.Equal(domains["test.default.svc.cluster-a.local."]) {
		t.Errorf("Got IP %s; want %s", aRec.A.String(), domains["test.default.svc.cluster-a.local."].String())
	}
}
