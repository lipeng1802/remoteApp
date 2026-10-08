package clientruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProductObservationRoleSchemas(t *testing.T) {
	target := `{"status":"count","pongs":3,"preauth_rejected":0,"sessions_closed":1}`
	controller := `{"status":"count","pongs":3,"ended":false,"result":"pending","pin_rejected":false}`
	for _, tc := range []struct {
		raw        string
		server, ok bool
	}{
		{target, true, true}, {controller, false, true}, {target, false, false}, {controller, true, false},
		{`{"status":"count","pongs":-1,"preauth_rejected":0,"sessions_closed":0}`, true, false},
		{`{"status":"count","pongs":0}`, true, false},
		{strings.Replace(target, `"pongs":3`, `"pongs":3,"Pongs":4`, 1), true, false},
		{strings.Replace(target, `"pongs":3`, `"pongs":3,"Key":"secret"`, 1), true, false},
		{strings.Replace(controller, `"pending"`, `"unknown"`, 1), false, false},
		{target + ` {}`, true, false},
	} {
		c, e := decodeProductCount([]byte(tc.raw), tc.server)
		if (e == nil) != tc.ok {
			t.Fatalf("schema server=%v accepted=%v", tc.server, e == nil)
		}
		if tc.ok {
			raw, _ := json.Marshal(c)
			if c.Status != "product_count" || c.Pongs != 3 || strings.Contains(string(raw), "Key") {
				t.Fatal("unsafe projection")
			}
		}
	}
}

func TestProductObservationUnavailableAndJoined(t *testing.T) {
	var absent *ProductObservation
	if _, e := absent.Count(context.Background()); e == nil {
		t.Fatal("nil observer accepted")
	}
	for _, server := range []bool{false, true} {
		a, pub := productFixture(t)
		cfg := a.c
		cfg.Server = server
		o := &ProductObservation{}
		if _, e := o.Count(context.Background()); e == nil {
			t.Fatal("unstarted observer accepted")
		}
		mode := "controller"
		if server {
			mode = "target"
		}
		p := Product{Spec: productSpec(t, mode), Observation: o}
		if server {
			p.TargetReady = func(string, string) {}
		} else {
			p.ControllerKey = testHMAC
		}
		child, e := p.start(context.Background(), a, pub, cfg, time.Now, 32123)
		if child != nil {
			t.Cleanup(func() { stopProduct(t, child) })
		}
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 2; i++ {
			c, e := o.Count(context.Background())
			if e != nil || c.Pongs != 3 {
				t.Fatal("actual child count", e)
			}
		}
		stopProduct(t, child)
		if _, e := o.Count(context.Background()); e == nil {
			t.Fatal("joined child queried")
		}
	}
}
