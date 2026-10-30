package startuptrace

import (
	"context"
	"io"
	"os"
	"testing"
)

func TestCompiledTESTGateAndFixedEnum(t *testing.T) {
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old; r.Close(); w.Close() }()
	Emit(nil, TargetGateDenied)
	Emit(context.Background(), ControllerDial)
	Emit(Context(context.Background(), false), TargetTLS)
	Emit(Context(context.Background(), true), Stage("PRIVATE"))
	Emit(Context(context.Background(), true), ControllerDial)
	w.Close()
	os.Stderr = old
	b, e := io.ReadAll(r)
	if e != nil || string(b) != "PRODUCT_STAGE controller_dial\n" {
		t.Fatal("TEST stage gate failed")
	}
}
