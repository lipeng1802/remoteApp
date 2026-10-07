package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func samePolicy(a, b []byte) bool {
	var x, y map[string]json.RawMessage
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil || len(x) != 1 || len(y) != 1 {
		return false
	}
	var xr, yr []any
	if json.Unmarshal(x["acls"], &xr) != nil || json.Unmarshal(y["acls"], &yr) != nil || len(xr) == 0 {
		return false
	}
	return reflect.DeepEqual(xr, yr)
}

func waitStopped(pid int) error {
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		b, e := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
		if e != nil {
			return denied
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "State:") && strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(line, "State:")), "T") {
				return nil
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return denied
}
