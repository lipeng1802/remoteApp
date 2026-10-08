package trustprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// Test adapter models durable bytes across owners. It is NOT an OS storage
// implementation, ACL test, authenticated seal or disk rollback protection.
type testJournal struct {
	raw       []byte
	loadErr   error
	commitErr bool
	uncertain bool
	commits   int
	onCommit  func()
}

func (j *testJournal) Load() ([]byte, error) {
	if j.loadErr != nil {
		return nil, j.loadErr
	}
	if j.raw == nil {
		return nil, ErrNotInstalled
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *testJournal) Commit(old, next []byte) error {
	if !bytes.Equal(old, j.raw) {
		return errors.New("cas failed")
	}
	if j.onCommit != nil {
		j.onCommit()
	}
	if j.commitErr && !j.uncertain {
		return errors.New("disk failed")
	}
	j.raw = append([]byte(nil), next...)
	j.commits++
	if j.commitErr {
		return errors.New("uncertain commit")
	}
	return nil
}

type quiesceFunc func(context.Context) error

func (f quiesceFunc) Quiesce(c context.Context) error { return f(c) }

func installationFixture(t *testing.T) (*Installation, testJournal, InstallationAnchor, Profile, Profile) {
	t.Helper()
	pub, key, claims := fixture(t)
	a := InstallationAnchor{Service: claims.Service, MinimumEpoch: 1}
	copy(a.Root[:], pub)
	old, _ := Sign(key, claims)
	claims.Epoch++
	claims.Endpoint = "https://next.example:8443"
	next, _ := Sign(key, claims)
	j := testJournal{}
	s, e := InitializeInstallation(a, old, &j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	// Return the journal copy only as persisted bytes; callers reopen with a
	// new owner and pointer, never use s together with this copied adapter.
	return s, j, a, old, next
}

func TestInstallationCommitBeforeUseAndDurableFloorOnReopen(t *testing.T) {
	_, j, a, old, next := installationFixture(t)
	s, e := OpenInstallation(a, &j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	joined := false
	j.onCommit = func() {
		if !joined {
			t.Fatal("commit before old workers joined")
		}
	}
	if e = s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { joined = true; return nil })); e != nil {
		t.Fatal(e)
	}
	if p, _, e := s.Current(); e != nil || p != next {
		t.Fatal("committed profile unavailable")
	}
	reopened, e := OpenInstallation(a, &j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if reopened.Accept(context.Background(), old, quiesceFunc(func(context.Context) error { t.Fatal("rollback cancelled live sessions"); return nil })) == nil {
		t.Fatal("floor reset after reopen")
	}
	if _, e = InitializeInstallation(a, old, &j, time.Now); e == nil {
		t.Fatal("reinstall overwrote prior state")
	}
}

func TestInstallationRejectsMissingCorruptWrongAnchorAndExternalMutation(t *testing.T) {
	_, j, a, _, _ := installationFixture(t)
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`{"version":1,"version":1}`), []byte(`{"version":1,"VERSION":1}`), append(append([]byte(nil), j.raw...), j.raw...)} {
		bad := testJournal{raw: raw}
		if _, e := OpenInstallation(a, &bad, time.Now); e == nil {
			t.Fatal("bad durable state accepted")
		}
	}
	wrong := a
	wrong.Root[0] ^= 1
	if _, e := OpenInstallation(wrong, &j, time.Now); e == nil {
		t.Fatal("journal supplied installation root")
	}
	wrong = a
	wrong.Service = "other"
	if _, e := OpenInstallation(wrong, &j, time.Now); e == nil {
		t.Fatal("journal supplied service")
	}
	s, _ := OpenInstallation(a, &j, time.Now)
	j.raw = []byte(`{}`)
	if _, _, e := s.Current(); e == nil {
		t.Fatal("external state change ignored")
	}
}

func TestInstallationJoinAndCommitFailuresLatchClosed(t *testing.T) {
	for _, mode := range []string{"join", "commit", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			_, j, a, old, next := installationFixture(t)
			s, _ := OpenInstallation(a, &j, time.Now)
			j.commitErr = mode != "join"
			j.uncertain = mode == "uncertain"
			e := s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error {
				if mode == "join" {
					return errors.New("unjoined")
				}
				return nil
			}))
			if e == nil {
				t.Fatal("unsafe rotation succeeded")
			}
			if _, _, e = s.Current(); e == nil {
				t.Fatal("failed owner handed out configuration")
			}
			if s.Accept(context.Background(), old, quiesceFunc(func(context.Context) error { return nil })) == nil {
				t.Fatal("failed owner auto recovered")
			}
			j.commitErr = false
			r, e := OpenInstallation(a, &j, time.Now)
			if e != nil {
				t.Fatal(e)
			}
			p, _, e := r.Current()
			if e != nil {
				t.Fatal(e)
			}
			want := old
			if mode == "uncertain" {
				want = next
			}
			if p != want {
				t.Fatal("recovery reset ambiguous durable commit")
			}
		})
	}
}

func TestInstallationExpiredRecoveryAndRaisedTrustedFloor(t *testing.T) {
	pub, key, c := fixture(t)
	a := InstallationAnchor{Service: c.Service, MinimumEpoch: 1}
	copy(a.Root[:], pub)
	start := time.Now().Truncate(time.Second)
	c.ExpiresAt = start.Add(time.Second).Unix()
	old, _ := Sign(key, c)
	now := start
	clock := func() time.Time { return now }
	j := &testJournal{}
	if _, e := InitializeInstallation(a, old, j, clock); e != nil {
		t.Fatal(e)
	}
	now = now.Add(2 * time.Second)
	s, e := OpenInstallation(a, j, clock)
	if e != nil {
		t.Fatal("expired trusted history must allow explicit rotation")
	}
	if _, _, e = s.Current(); e == nil {
		t.Fatal("expired profile used")
	}
	c.Epoch++
	c.ExpiresAt = now.Add(time.Hour).Unix()
	next, _ := Sign(key, c)
	if s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })) != nil {
		t.Fatal("valid explicit expired recovery refused")
	}
	a.MinimumEpoch = c.Epoch + 1
	r, e := OpenInstallation(a, j, clock)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = r.Current(); e == nil {
		t.Fatal("raised installation floor ignored")
	}
	c.Epoch++
	next, _ = Sign(key, c)
	if r.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })) != nil {
		t.Fatal("higher floor recovery refused")
	}
}

func TestInstallationRechecksAfterJoinAndRejectsSameEpochMutation(t *testing.T) {
	pub, key, c := fixture(t)
	a := InstallationAnchor{Service: c.Service, MinimumEpoch: 1}
	copy(a.Root[:], pub)
	old, _ := Sign(key, c)
	j := &testJournal{}
	now := time.Now()
	clock := func() time.Time { return now }
	s, e := InitializeInstallation(a, old, j, clock)
	if e != nil {
		t.Fatal(e)
	}
	c.Endpoint = "https://changed.example:8443"
	mutation, _ := Sign(key, c)
	if s.Accept(context.Background(), mutation, quiesceFunc(func(context.Context) error { t.Fatal("invalid profile joined sessions"); return nil })) == nil {
		t.Fatal("same epoch mutation accepted")
	}
	c.Epoch++
	next, _ := Sign(key, c)
	commits := j.commits
	if s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { now = time.Unix(c.ExpiresAt, 0); return nil })) == nil || j.commits != commits {
		t.Fatal("expired during join committed")
	}
	var record installationRecord
	if json.Unmarshal(j.raw, &record) != nil || record.Profile != old {
		t.Fatal("failed rotation overwrote durable state")
	}
}
