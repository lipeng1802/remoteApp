package main

import (
	"fmt"
	"os"
	"remoteapp.local/selfhost-poc/prerelease"
	"time"
)

func main() {
	if len(os.Args) != 4 || prerelease.Create(os.Args[1], os.Args[2], os.Args[3], time.Now()) != nil {
		fmt.Println("FAIL prerelease bootstrap; existing/partial state is preserved")
		os.Exit(1)
	}
	fmt.Println("READY pre-release material created; no deployment or trust installation")
}
