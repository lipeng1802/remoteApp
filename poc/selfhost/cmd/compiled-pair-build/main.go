// Trusted isolated test build, NOT runtime bootstrap or production root supply.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Println("BUILD_FAILED: partial test material preserved; no installation or network started")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY: isolated matching compiled pair; NOT installed or cross-machine accepted")
}
func publicInput(path string) (trustinstall.TestPairInput, error) {
	f, e := os.Open(path)
	if e != nil {
		return trustinstall.TestPairInput{}, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 8193))
	if e != nil || len(b) > 8192 {
		return trustinstall.TestPairInput{}, os.ErrInvalid
	}
	reader := privatepipe.NewReader(bytes.NewReader(append(bytes.TrimSpace(b), '\n')))
	var in trustinstall.TestPairInput
	var extra struct{}
	if reader.Read(&in) != nil || reader.Read(&extra) != io.EOF {
		return in, os.ErrInvalid
	}
	return in, nil
}
func run() error {
	names := []string{"go", "module", "input", "out", "mac-journal", "win-journal", "mac-vault", "win-vault", "mac-product", "win-product"}
	flags := flag.NewFlagSet("compiled-pair-build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	args := map[string]*string{}
	for _, name := range names {
		args[name] = flags.String(name, "", "")
	}
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 0 {
		return os.ErrInvalid
	}
	for _, name := range names {
		p := *args[name]
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	in, e := publicInput(*args["input"])
	if e != nil {
		return e
	}
	material, e := trustinstall.NewTestPairMaterial(in, time.Now())
	if e != nil {
		return e
	}
	dest := *args["out"]
	parent := filepath.Dir(dest)
	real, e := filepath.EvalSymlinks(parent)
	if e != nil || real != parent {
		return os.ErrInvalid
	}
	if e = os.Mkdir(dest, 0700); e != nil {
		return e
	}
	files, e := material.TestPairPublicFiles()
	if e != nil {
		return e
	}
	for name, b := range files {
		f, e := os.OpenFile(filepath.Join(dest, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = f.Write(b)
		if e == nil {
			e = f.Sync()
		}
		if closed := f.Close(); e == nil {
			e = closed
		}
		if e != nil {
			return e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	for _, platform := range []string{"mac", "win"} {
		target, profile := "darwin-test", "controller-profile.json"
		if platform == "win" {
			target, profile = "windows-desktop-borrowed-test", "target-profile.json"
		}
		command := exec.CommandContext(ctx, *args["go"], "run", "./cmd/installed-session-build", *args["go"], *args["module"], target, filepath.Join(dest, platform), filepath.Join(dest, "installation-anchor.json"), filepath.Join(dest, profile), *args[platform+"-journal"], *args[platform+"-vault"], "compiled-pair-test", *args[platform+"-product"])
		command.Dir = *args["module"]
		command.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off")
		if command.Run() != nil {
			return os.ErrInvalid
		}
		// Audit both bundles are test-only and actually pin the expected public
		// service profile, not whichever old file happened to be nearby.
		b, e := os.ReadFile(filepath.Join(dest, platform, "build-public.json"))
		if e != nil {
			return e
		}
		var built struct {
			Release trustinstall.Specification `json:"release"`
		}
		if json.Unmarshal(b, &built) != nil || !built.Release.Test || built.Release.Root != material.Anchor.Root {
			return os.ErrInvalid
		}
		wanted := material.Controller
		if platform == "win" {
			wanted = material.Target
		}
		if built.Release.Bundled != wanted {
			return os.ErrInvalid
		}
	}
	return nil
}
