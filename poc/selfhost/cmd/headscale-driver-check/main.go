// Server-local administrator check. Emits no keys, IDs or provider output.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
)

func main() {
	cleanup := flag.String("cleanup-ticket", "", "recover one explicitly identified test ticket")
	flag.Parse()
	d := &headscale.Driver{Run: headscale.LocalRunner()}
	if *cleanup != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if d.Cleanup(ctx, invite.ProvisionRecord{ID: *cleanup}) != nil {
			fmt.Println("FAIL ticket_cleanup")
			os.Exit(1)
		}
		fmt.Println("PASS ticket_cleanup")
		return
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		os.Exit(1)
	}
	r := invite.ProvisionRecord{ID: hex.EncodeToString(b), Expires: time.Now().Unix() + 90}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	c, e := d.Mint(ctx, r)
	duplicate, duplicateErr := d.Mint(ctx, r)
	// Cleanup by ticket works even if mint failed before returning handles.
	clean := d.Cleanup(ctx, r)
	if e != nil || c.Secret == "" || clean != nil || duplicateErr == nil || duplicate.Secret != "" {
		if e != nil {
			fmt.Println("FAIL mint_validation")
		}
		if clean != nil {
			fmt.Println("FAIL scoped_cleanup")
		}
		fmt.Println("FAIL real_driver_mint_cleanup")
		os.Exit(1)
	}
	if d.Cleanup(ctx, r) != nil {
		fmt.Println("FAIL cleanup_idempotent")
		os.Exit(1)
	}
	fmt.Println("PASS real_driver_single_use_absolute_expiry_cleanup")
}
