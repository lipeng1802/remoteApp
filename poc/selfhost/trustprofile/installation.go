package trustprofile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

// ErrNotInstalled may be returned ONLY for an explicitly new installation,
// not as a recovery fallback for deleted, corrupt or inaccessible state.
var ErrNotInstalled = errors.New("trust_not_installed")

var ErrMaintenance = errors.New("trust_maintenance_required")

type maintenanceRecord struct {
	Version int    `json:"version"`
	State   string `json:"state"`
	ID      string `json:"id"`
}

// MaintenanceState is diagnostic only, never a usable profile or deletion permit.
type MaintenanceState struct {
	State       string
	Epoch       uint64
	Transaction string
}

// InstallationAnchor comes from the trusted installer/binary, never a network
// manifest, journal, peer or endpoint. Root rotation needs a separate ceremony.
type InstallationAnchor struct {
	Root         [32]byte
	Service      string
	MinimumEpoch uint64
}

// TrustJournal is a platform protection boundary, not a plain-file fallback.
// The adapter must hold an exclusive owner lease for this Installation's entire
// lifetime and protect against unauthorized reset/rollback. Commit is atomic,
// compare-and-swap and durable before returning nil. Uncertain failure must
// return an error. A production OS adapter is deliberately not provided here.
type TrustJournal interface {
	Load() ([]byte, error)
	Commit(expected, next []byte) error
}

// SessionQuiescer must cancel and JOIN every old connection/watch/tunnel worker.
// An error means no new configuration may be used, even if cancellation began.
// Quiesce must respect its context and must not reenter this Installation.
type SessionQuiescer interface{ Quiesce(context.Context) error }

type installationRecord struct {
	Version     int                `json:"version"`
	Anchor      string             `json:"anchor"`
	Epoch       uint64             `json:"epoch"`
	AcceptedAt  int64              `json:"accepted_at"`
	Profile     Profile            `json:"profile"`
	Maintenance *maintenanceRecord `json:"maintenance,omitempty"`
}

type Installation struct {
	mu      sync.Mutex
	anchor  InstallationAnchor
	journal TrustJournal
	clock   func() time.Time
	record  installationRecord
	raw     []byte
	broken  bool
}

func anchorID(a InstallationAnchor) (string, error) {
	if a.Root == ([32]byte{}) || a.Service == "" || len(a.Service) > 64 || strings.ContainsAny(a.Service, "\r\n\x00") || a.MinimumEpoch == 0 || a.MinimumEpoch > 1<<53-1 {
		return "", ErrTrust
	}
	// The installed root/service identity is stable; a trusted software update
	// may raise MinimumEpoch without silently replacing an existing journal.
	b, _ := json.Marshal(struct {
		Root    [32]byte
		Service string
	}{a.Root, a.Service})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func decodeInstallation(raw []byte) (installationRecord, error) {
	r := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')))
	var record installationRecord
	if len(raw) == 0 || len(raw) > 8191 || r.Read(&record) != nil {
		return record, ErrTrust
	}
	var extra struct{}
	if r.Read(&extra) != io.EOF {
		return record, ErrTrust
	}
	if record.Maintenance == nil {
		// Absence is the legacy active format. Explicit null is a malformed
		// maintenance record, not permission to fall back to active state.
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return record, ErrTrust
		}
		for name := range fields {
			if strings.EqualFold(name, "maintenance") {
				return record, ErrTrust
			}
		}
	}
	if m := record.Maintenance; m != nil {
		id, e := hex.DecodeString(m.ID)
		if e != nil || len(id) != 16 || hex.EncodeToString(id) != m.ID || m.Version != 1 || m.State != "pending" {
			return record, ErrTrust
		}
	}
	return record, nil
}

func encodeInstallation(r installationRecord) ([]byte, error) {
	b, e := json.Marshal(r)
	if e != nil || len(b) > 8191 {
		return nil, ErrTrust
	}
	return b, nil
}

// InitializeInstallation is installer-only and refuses any prior record. It
// never creates a root, signs a profile, deletes state or repairs a lost journal.
func InitializeInstallation(a InstallationAnchor, p Profile, j TrustJournal, clock func() time.Time) (*Installation, error) {
	id, e := anchorID(a)
	if e != nil || j == nil || clock == nil {
		return nil, ErrTrust
	}
	prior, e := j.Load()
	if !errors.Is(e, ErrNotInstalled) || len(prior) != 0 {
		return nil, ErrTrust
	}
	now := clock()
	if _, e = Verify(p, a.Root[:], a.Service, a.MinimumEpoch, now); e != nil {
		return nil, ErrTrust
	}
	r := installationRecord{Version: 1, Anchor: id, Epoch: p.Claims.Epoch, AcceptedAt: now.Unix(), Profile: p}
	raw, e := encodeInstallation(r)
	if e != nil || j.Commit(nil, raw) != nil {
		return nil, ErrTrust
	}
	return OpenInstallation(a, j, clock)
}

// Open validates persisted authority historically, even if a profile expired.
// Current still rejects expired profiles; only an explicit valid newer Accept
// can recover. No missing/corrupt-state initialization fallback is permitted.
func OpenInstallation(a InstallationAnchor, j TrustJournal, clock func() time.Time) (*Installation, error) {
	r, raw, e := readInstallation(a, j, clock)
	if e != nil {
		return nil, e
	}
	if r.Maintenance != nil {
		return nil, ErrMaintenance
	}
	return &Installation{anchor: a, journal: j, clock: clock, record: r, raw: append([]byte(nil), raw...)}, nil
}

func readInstallation(a InstallationAnchor, j TrustJournal, clock func() time.Time) (installationRecord, []byte, error) {
	id, e := anchorID(a)
	if e != nil || j == nil || clock == nil {
		return installationRecord{}, nil, ErrTrust
	}
	raw, e := j.Load()
	if e != nil {
		return installationRecord{}, nil, ErrTrust
	}
	r, e := decodeInstallation(raw)
	if e != nil || r.Version != 1 || r.Anchor != id || r.Epoch != r.Profile.Claims.Epoch || r.AcceptedAt < r.Profile.Claims.IssuedAt || r.AcceptedAt >= r.Profile.Claims.ExpiresAt || clock().Unix() < r.AcceptedAt {
		return installationRecord{}, nil, ErrTrust
	}
	if _, e = Verify(r.Profile, a.Root[:], a.Service, r.Epoch, time.Unix(r.AcceptedAt, 0)); e != nil {
		return installationRecord{}, nil, ErrTrust
	}
	return r, raw, nil
}

// ReadMaintenance validates historical signed authority but never opens a
// session or returns the profile. Missing/corrupt records are not repairable here.
func ReadMaintenance(a InstallationAnchor, j TrustJournal, clock func() time.Time) (MaintenanceState, error) {
	r, _, err := readInstallation(a, j, clock)
	if err != nil {
		return MaintenanceState{}, err
	}
	if r.Maintenance == nil {
		return MaintenanceState{}, ErrTrust
	}
	return MaintenanceState{State: r.Maintenance.State, Epoch: r.Epoch, Transaction: r.Maintenance.ID}, nil
}

// BeginMaintenance records intent under the live owner's exclusive lease
// before cancellation/join. External maintenance must still wait for join.
// It preserves the exact accepted profile/floor.
// An uncertain receipt latches this Installation closed even if the marker won.
func (s *Installation) BeginMaintenance() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || s.record.Maintenance != nil || !s.unchanged() || s.clock().Unix() < s.record.AcceptedAt {
		return ErrTrust
	}
	if _, e := Verify(s.record.Profile, s.anchor.Root[:], s.anchor.Service, s.floor(), s.clock()); e != nil {
		return ErrTrust
	}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return ErrTrust
	}
	r := s.record
	r.Maintenance = &maintenanceRecord{Version: 1, State: "pending", ID: hex.EncodeToString(nonce[:])}
	raw, e := encodeInstallation(r)
	if e != nil || s.journal.Commit(s.raw, raw) != nil {
		s.broken = true
		return ErrTrust
	}
	s.record = r
	s.raw = raw
	if !s.unchanged() {
		return ErrTrust
	}
	return nil
}

func (s *Installation) unchanged() bool {
	b, e := s.journal.Load()
	if e != nil || !bytes.Equal(b, s.raw) {
		s.broken = true
		return false
	}
	return true
}

func (s *Installation) floor() uint64 {
	if s.anchor.MinimumEpoch > s.record.Epoch {
		return s.anchor.MinimumEpoch
	}
	return s.record.Epoch
}

// Current exposes no usable profile until installation/rotation is committed.
// Expiry or a raised installer floor never falls back to the previous profile.
func (s *Installation) Current() (Profile, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if s.broken || s.record.Maintenance != nil || !s.unchanged() || now.Unix() < s.record.AcceptedAt {
		return Profile{}, nil, ErrTrust
	}
	cert, e := Verify(s.record.Profile, s.anchor.Root[:], s.anchor.Service, s.floor(), now)
	if e != nil {
		return Profile{}, nil, ErrTrust
	}
	return s.record.Profile, cert, nil
}

// Accept verifies first, joins old sessions, rechecks time, and commits before
// publication. Join/CAS/durability failures latch this owner closed; explicitly
// reopen trusted durable state after inspection, never auto-reconnect/reset.
func (s *Installation) Accept(ctx context.Context, p Profile, sessions SessionQuiescer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil || ctx.Err() != nil || s.broken || s.record.Maintenance != nil || !s.unchanged() || s.clock().Unix() < s.record.AcceptedAt {
		return ErrTrust
	}
	if _, e := Advance(s.record.Profile, p, s.anchor.Root[:], s.anchor.Service, s.floor(), s.clock()); e != nil {
		return ErrTrust
	}
	if p == s.record.Profile {
		return nil
	}
	if sessions == nil {
		return ErrTrust
	}
	if sessions.Quiesce(ctx) != nil || ctx.Err() != nil {
		s.broken = true
		return ErrTrust
	}
	now := s.clock()
	if now.Unix() < s.record.AcceptedAt {
		s.broken = true
		return ErrTrust
	}
	if _, e := Advance(s.record.Profile, p, s.anchor.Root[:], s.anchor.Service, s.floor(), now); e != nil {
		s.broken = true
		return ErrTrust
	}
	r := installationRecord{Version: 1, Anchor: s.record.Anchor, Epoch: p.Claims.Epoch, AcceptedAt: now.Unix(), Profile: p}
	raw, e := encodeInstallation(r)
	if e != nil || s.journal.Commit(s.raw, raw) != nil {
		s.broken = true
		return ErrTrust
	}
	s.record = r
	s.raw = raw
	if !s.unchanged() {
		return ErrTrust
	}
	return nil
}
