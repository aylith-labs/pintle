package logger

import (
	"bytes"
	"regexp"
	"testing"
	"time"
)

func TestAccessKeepsRouteShapeAndAppendsDurationAndSize(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()
	Access("GET", "app.lvh.me", "/hello", "http://localhost:3000", 200, 1500*time.Microsecond, 2048, false)
	Access("GET", "app.lvh.me", "/large", "http://localhost:3000", 200, 2*time.Second, 10, true)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(buf.String(), "")
	want := regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d 200   GET app\.lvh\.me/hello -> http://localhost:3000 1\.5ms 2\.0KB\n` +
		`\d\d:\d\d:\d\d 200   GET app\.lvh\.me/large -> http://localhost:3000 2\.00s 10B aborted\n$`)
	if !want.MatchString(plain) {
		t.Fatalf("unexpected lines:\n%s", plain)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{0: "0B", 1023: "1023B", 1024: "1.0KB", 80 * 1024 * 1024: "80.0MB"}
	for size, want := range cases {
		if got := FormatBytes(size); got != want {
			t.Errorf("FormatBytes(%d) = %q; want %q", size, got, want)
		}
	}
}
