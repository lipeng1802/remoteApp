// Private test controller. Stdout carries credentials ONLY into the parent
// pipe; never launch interactively or print its responses in logs.
package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

type command struct {
	Op      string                `json:"op"`
	Binding invite.NetworkBinding `json:"binding"`
}

func main() {
	if len(os.Args) != 1 || run() != nil {
		os.Exit(1)
	}
}
func run() error {
	root, err := filepath.Abs("../../artifacts/connection-poc/invitation-fixtures")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(root, "grant-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) // exact newly-created test directory only
	store, err := invite.Open(filepath.Join(dir, "state"), time.Now)
	if err != nil {
		return err
	}
	defer func() {
		if store != nil {
			store.Close()
		}
	}()
	_, target, _ := ed25519.GenerateKey(rand.Reader)
	_, client, _ := ed25519.GenerateKey(rand.Reader)
	exec := func(k ed25519.PrivateKey, b invite.Body) (invite.Result, error) {
		n := make([]byte, 16)
		if _, e := rand.Read(n); e != nil {
			return invite.Result{}, e
		}
		b.Nonce = hex.EncodeToString(n)
		b.IssuedAt = time.Now().Unix()
		return store.Execute(invite.Sign(k, b))
	}
	a, err := exec(target, invite.Body{Action: "register"})
	if err != nil {
		return err
	}
	if _, err = exec(client, invite.Body{Action: "register"}); err != nil {
		return err
	}
	create := func() (invite.Grant, error) {
		inv, e := exec(target, invite.Body{Action: "invite"})
		if e != nil {
			return invite.Grant{}, e
		}
		req, e := exec(client, invite.Body{Action: "claim", TargetCode: a.DeviceCode, Code: inv.Code})
		if e != nil {
			return invite.Grant{}, e
		}
		g, e := exec(target, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: true})
		if e != nil {
			return invite.Grant{}, e
		}
		return *g.Grant, nil
	}
	g, err := create()
	if err != nil {
		return err
	}
	var binding invite.NetworkBinding
	r := bufio.NewReaderSize(os.Stdin, 4098)
	for {
		line, e := r.ReadSlice('\n')
		if e == io.EOF && len(line) == 0 {
			return nil
		}
		if e != nil {
			return e
		}
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		var c command
		if e = d.Decode(&c); e != nil {
			return e
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return invite.ErrDenied
		}
		switch c.Op {
		case "bind":
			binding = c.Binding
		case "lease":
		case "revoke":
			if _, e = exec(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID}); e != nil {
				return e
			}
		case "restart":
			if e = store.Close(); e != nil {
				return e
			}
			store, e = invite.Open(filepath.Join(dir, "state"), time.Now)
			if e != nil {
				return e
			}
		case "renew":
			g, e = create()
			if e != nil {
				return e
			}
		default:
			return invite.ErrDenied
		}
		n, e := store.NetworkState(g, binding)
		if e != nil {
			return e
		}
		base := map[string]any{"issuer": hex.EncodeToString(store.PublicKey()), "target": hex.EncodeToString(target.Public().(ed25519.PublicKey)), "controller": hex.EncodeToString(client.Public().(ed25519.PublicKey)), "state": n}
		server := map[string]any{}
		probe := map[string]any{}
		for k, v := range base {
			server[k] = v
			probe[k] = v
		}
		server["private_key"] = hex.EncodeToString(target)
		probe["private_key"] = hex.EncodeToString(client)
		if e = json.NewEncoder(os.Stdout).Encode(map[string]any{"server": server, "probe": probe, "state": n}); e != nil {
			return e
		}
	}
}
