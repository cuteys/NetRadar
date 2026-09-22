package collector

import "testing"

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"192.168.254.254", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"114.114.114.114", false},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"fd00::1", true},
		{"240e::1", false},
		{"2001:4860:4860::8888", false},
		{"invalid", false},
	}

	for _, c := range cases {
		got := IsPrivateIP(c.ip)
		if got != c.expected {
			t.Errorf("IsPrivateIP(%q) = %v, expected %v", c.ip, got, c.expected)
		}
	}
}
