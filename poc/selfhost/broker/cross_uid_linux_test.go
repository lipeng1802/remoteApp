//go:build linux

package broker

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestBrokerCrossUIDBoundary(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires isolated root Linux test executor")
	}
	u, e := user.Lookup("nobody")
	if e != nil {
		t.Fatal(e)
	}
	gid, e := strconv.ParseUint(u.Gid, 10, 32)
	if e != nil || u.Uid == "0" {
		t.Fatal("non-root test user")
	}
	parent, e := os.MkdirTemp("", "rb-cross-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(parent)
	if os.Chmod(parent, 0755) != nil {
		t.Fatal("parent permissions")
	}
	dir := filepath.Join(parent, "socket")
	if os.Mkdir(dir, 0710) != nil || os.Chown(dir, 0, int(gid)) != nil {
		t.Fatal("socket directory")
	}
	path := filepath.Join(dir, "authority.sock")
	executor := &counter{}
	cancel, done := start(t, Config{Socket: path, PeerUID: 0, GroupID: uint32(gid), Executor: executor})
	defer cancel()
	self, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	src, e := os.Open(self)
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	program := filepath.Join(parent, "peer-test")
	out, e := os.OpenFile(program, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if e != nil {
		t.Fatal(e)
	}
	_, e = io.Copy(out, src)
	ce := out.Close()
	if e != nil || ce != nil {
		t.Fatal("copy test program")
	}
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	command := exec.CommandContext(ctx, "runuser", "-u", "nobody", "--", program, "-test.run=^TestBrokerCrossUIDClient$", "-test.v=true")
	command.Env = append(os.Environ(), "REMOTEAPP_BROKER_TEST_SOCKET="+path)
	if output, e := command.CombinedOutput(); e != nil {
		t.Fatalf("cross-user child failed: %s", output)
	}
	stop(t, cancel, done)
	if executor.calls.Load() != 0 {
		t.Fatal("wrong kernel uid reached authority")
	}
}
func TestBrokerCrossUIDClient(t *testing.T) {
	path := os.Getenv("REMOTEAPP_BROKER_TEST_SOCKET")
	if path == "" {
		t.Skip("helper subprocess only")
	}
	if os.Geteuid() == 0 {
		t.Fatal("not cross-user")
	}
	conn, e := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		t.Fatal("OS socket access must succeed before UID rejection")
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	uid, e := peerUID(conn)
	if e != nil || uid != 0 {
		t.Fatal("wrong server kernel uid")
	}
	// Write can race the deliberate server close. In either case no authority
	// reply may be returned to this otherwise group-permitted different UID.
	send(conn, Request{Operation: "device"})
	conn.CloseWrite()
	var r Reply
	if receive(conn, &r) == nil {
		t.Fatal("cross-user request accepted")
	}
}
