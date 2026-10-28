package installeradapter

import (
	"errors"
	"reflect"
	"testing"
)

func TestFlowOrdersCommitAndReopen(t *testing.T) {
	for _, op := range []string{"install", "upgrade"} {
		t.Run(op, func(t *testing.T) {
			var calls []string
			transactions, bootstraps := 0, 0
			err := Run(op, 2, func(tool string, args ...string) (Reply, error) {
				verb := tool
				if len(args) > 0 {
					verb += ":" + args[0]
				}
				calls = append(calls, verb)
				if tool == "install-transaction" {
					transactions++
					if transactions == 1 {
						return Reply{}, errors.New("uncertain")
					}
					phase := "staged"
					if transactions > 2 {
						phase = "promoted"
					}
					return Reply{Status: "prepared", Epoch: 2, Phase: phase}, nil
				}
				if tool == "installed-bootstrap-manager" && len(args) == 0 {
					bootstraps++
					if bootstraps < 4 {
						return Reply{}, errors.New("uncertain")
					}
				}
				return Reply{Status: "active", Epoch: 2}, nil
			})
			want := []string{"install-transaction", "install-transaction", "installed-release-manager:" + op, "install-transaction", "installed-bootstrap-manager", "installed-bootstrap-manager", "installed-bootstrap-manager", "installed-bootstrap-manager", "installed-release-manager:status", "installed-bootstrap-manager:status"}
			if err != nil || !reflect.DeepEqual(calls, want) {
				t.Fatalf("err=%v calls=%v", err, calls)
			}
		})
	}
}

func TestFlowRejectsEachBrokenBoundary(t *testing.T) {
	for broken := 0; broken < 6; broken++ {
		t.Run(string(rune('A'+broken)), func(t *testing.T) {
			step, calls := 0, 0
			err := Run("install", 1, func(tool string, args ...string) (Reply, error) {
				calls++
				current := step
				if current == broken {
					return Reply{Status: "active", Epoch: 999}, nil
				}
				step++
				if tool == "install-transaction" {
					phase := "staged"
					if current > 0 {
						phase = "promoted"
					}
					return Reply{Status: "prepared", Epoch: 1, Phase: phase}, nil
				}
				return Reply{Status: "active", Epoch: 1}, nil
			})
			if err == nil || step != broken || calls > 9 {
				t.Fatalf("err=%v step=%d calls=%d", err, step, calls)
			}
		})
	}
}

func TestInvalidConfigurationDoesNotInvoke(t *testing.T) {
	for _, op := range []string{"", "uninstall", "reset"} {
		if Run(op, 1, func(string, ...string) (Reply, error) { t.Fatal("invoked"); return Reply{}, nil }) == nil {
			t.Fatal("accepted", op)
		}
	}
	if Run("install", 0, nil) == nil {
		t.Fatal("zero epoch")
	}
}
