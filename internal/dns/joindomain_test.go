package dns

import (
	"testing"
)

func TestJoinDomain(t *testing.T) {
	testCases := []struct {
		name     string
		domain   string
		search   string
		expected string
	}{
		{
			name:     "simple domain with search",
			domain:   "test",
			search:   "default.svc.cluster-a.local",
			expected: "test.default.svc.cluster-a.local.",
		},
		{
			name:     "domain with partial match of search",
			domain:   "test.default",
			search:   "default.svc.cluster-a.local",
			expected: "test.default.svc.cluster-a.local.",
		},
		{
			name:     "domain already has full search",
			domain:   "test.default.svc.cluster-a.local",
			search:   "default.svc.cluster-a.local",
			expected: "test.default.svc.cluster-a.local.",
		},
		{
			name:     "domain with dots already",
			domain:   "simple-server.default",
			search:   "default.svc.cluster-a.local",
			expected: "simple-server.default.svc.cluster-a.local.",
		},
		{
			name:     "domain and search with trailing dots",
			domain:   "simple-server.default.",
			search:   "default.svc.cluster-a.local.",
			expected: "simple-server.default.svc.cluster-a.local.",
		},
		{
			name:     "no overlap",
			domain:   "service.ns1",
			search:   "ns2.svc.local",
			expected: "service.ns1.ns2.svc.local.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := JoinDomain(tc.domain, tc.search)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}
