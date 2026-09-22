package collector

import "testing"

func TestParseConntrackLine(t *testing.T) {
	line := "ipv4 2 tcp 6 299 ESTABLISHED src=192.168.1.100 dst=1.1.1.1 sport=54321 dport=443 packets=10 bytes=1200 src=1.1.1.1 dst=192.168.1.100 sport=443 dport=54321 packets=12 bytes=2400 [ASSURED] mark=0 use=1"
	
	entry := parseConntrackLine(line)
	if entry == nil {
		t.Fatalf("parseConntrackLine failed on valid input")
	}

	if entry.Proto != "TCP" {
		t.Errorf("Proto = %q, want TCP", entry.Proto)
	}
	if entry.Src1 != "192.168.1.100" {
		t.Errorf("Src1 = %q, want 192.168.1.100", entry.Src1)
	}
	if entry.Dst1 != "1.1.1.1" {
		t.Errorf("Dst1 = %q, want 1.1.1.1", entry.Dst1)
	}
	if entry.Sport1 != 54321 {
		t.Errorf("Sport1 = %d, want 54321", entry.Sport1)
	}
	if entry.Dport1 != 443 {
		t.Errorf("Dport1 = %d, want 443", entry.Dport1)
	}
	if entry.Bytes1 != 1200 {
		t.Errorf("Bytes1 = %d, want 1200", entry.Bytes1)
	}
	if entry.Bytes2 != 2400 {
		t.Errorf("Bytes2 = %d, want 2400", entry.Bytes2)
	}
}

func TestGenerateFlowKey(t *testing.T) {
	key := GenerateFlowKey("TCP", "192.168.1.100", 54321, "1.1.1.1", 443)
	expected := "TCP:1.1.1.1:443-192.168.1.100:54321"
	if key != expected {
		t.Errorf("GenerateFlowKey = %q, want %q", key, expected)
	}
}
