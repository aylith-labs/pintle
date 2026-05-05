package hostdetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadHostRoute(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "WSL-like default via 172.22.144.1",
			content: "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
				"eth0\t00000000\t019016AC\t0003\t0\t0\t0\t00000000\t0\t0\t0\n" +
				"eth0\t009016AC\t00000000\t0001\t0\t0\t0\t00F0FFFF\t0\t0\t0\n",
			want: "172.22.144.1",
		},
		{
			name: "docker0 default via 172.16.0.1",
			content: "Iface\tDestination\tGateway\tFlags\n" +
				"eth0\t00000000\t010010AC\t0003\n",
			want: "172.16.0.1",
		},
		{
			name:    "no default route",
			content: "Iface\tDestination\tGateway\tFlags\n",
			want:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "route")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got := readHostRoute(p)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseHexIPLE(t *testing.T) {
	tests := []struct {
		hex  string
		want string
		ok   bool
	}{
		{"019016AC", "172.22.144.1", true},
		{"010010AC", "172.16.0.1", true},
		{"0100007F", "127.0.0.1", true},
		{"short", "", false},
		{"01234567XY", "", false},
	}
	for _, tc := range tests {
		got, ok := parseHexIPLE(tc.hex)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseHexIPLE(%q) = (%q, %v), want (%q, %v)", tc.hex, got, ok, tc.want, tc.ok)
		}
	}
}
