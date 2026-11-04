package main

import (
	"fmt"
	"os"
	"remoteapp.local/selfhost-poc/archivecache"
)

func main() {
	if len(os.Args) != 4 || archivecache.Restore(os.Args[1], os.Args[2], os.Args[3]) != nil {
		fmt.Println("ARCHIVE_CACHE_REJECTED")
		os.Exit(1)
	}
	fmt.Println("ARCHIVE_CACHE_RESTORED")
}
