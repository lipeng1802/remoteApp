package privatepipe

import (
	"io"
	"strings"
	"testing"
)

func TestStrictBoundedFrames(t *testing.T) {
	for _, raw := range []string{"{\"Op\":\"stop\",\"op\":\"start\"}\n", "{\"Op\":\"stop\",\"Extra\":1}\n", "{} {}\n", "{\"Op\":\"stop\"}", strings.Repeat(" ", 8192) + "{}\n", "{\"Data\":{\"x\":1,\"x\":2}}\n"} {
		var v struct {
			Op   string
			Data map[string]int
		}
		if NewReader(strings.NewReader(raw)).Read(&v) != ErrProtocol {
			t.Fatal("accepted invalid frame")
		}
	}
	r := NewReader(strings.NewReader("{\"Op\":\"count\"}\n{\"Op\":\"stop\"}\n"))
	var v struct{ Op string }
	if r.Read(&v) != nil || v.Op != "count" {
		t.Fatal("frame1")
	}
	if r.Read(&v) != nil || v.Op != "stop" {
		t.Fatal("frame2")
	}
	if r.Read(&v) != io.EOF {
		t.Fatal("EOF")
	}
}
