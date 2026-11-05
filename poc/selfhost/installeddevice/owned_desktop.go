package installeddevice

import (
	"context"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// OwnedDesktopSession binds the native entry to one installed GUI and role.
// The private management channel can start the flow once and observe progress.
func OwnedDesktopSession(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request,
	output func(any) error, factory EnrollmentFactory, product clientruntime.Product,
	expected invite.DesktopPermission, target bool, port uint16) clientowner.Runner {
	policy, err := invite.DesktopGrantPolicy(expected)
	if err != nil || !release.Specification().Test || factory == nil || (port != 0 && port < 1024) ||
		product.Prepared != nil || product.Observation != nil {
		return func(context.Context, trustprofile.Profile) error { return trustprofile.ErrTrust }
	}
	return runner(release, open, commands, output, factory, true, true, nil, runnerOptions{
		testTunnel: port, coordinated: true, policy: policy,
		desktop: &desktopProduct{product: product, target: target},
	})
}

type desktopProduct struct {
	product clientruntime.Product
	target  bool
}

func ValidDesktopLifecycle(r Request) bool {
	switch r.Op {
	case "desktop_begin", "desktop_status", "desktop_state", "desktop_arm_stop":
		return r == (Request{Op: r.Op})
	default:
		return false
	}
}

// DesktopProgress travels only on the private supervisor channel. The grant
// is the verified reply for RequestID, never invitation or TLS material.
type DesktopProgress struct {
	Status    string        `json:"status"`
	Phase     string        `json:"phase"`
	RequestID string        `json:"request_id,omitempty"`
	Grant     *invite.Grant `json:"grant,omitempty"`
}

func (p DesktopProgress) Valid() bool {
	if p.Status != "desktop_status" || (p.RequestID != "" && !canonical(p.RequestID, 16)) {
		return false
	}
	switch p.Phase {
	case "idle":
		return p.RequestID == "" && p.Grant == nil
	case "waiting", "fault", "stopped":
		return p.Grant == nil
	case "denied", "expired", "revoked", "declined":
		return p.RequestID != "" && p.Grant == nil
	case "approved":
		return p.RequestID != "" && p.Grant != nil
	default:
		return false
	}
}

type desktopFlow struct {
	desktopProduct
	mu        sync.Mutex
	progress  DesktopProgress
	cancel    context.CancelFunc
	done      chan struct{}
	stopArmed bool
	prepared  *clientruntime.PreparedDesktop // published before done closes
}

func (d *desktopFlow) snapshot() DesktopProgress {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.progress
}

func (d *desktopFlow) set(phase, request string, grant *invite.Grant) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progress = DesktopProgress{Status: "desktop_status", Phase: phase, RequestID: request, Grant: grant}
}

func (d *desktopFlow) begin(ctx context.Context, gate *authorizationGate, client *invite.NetworkClient) {
	life, cancel := client.BoundLifetime(ctx)
	d.cancel, d.done = cancel, make(chan struct{})
	d.set("waiting", "", nil)
	go func() {
		phase, request, grant := d.prepare(life, gate, client)
		if life.Err() != nil {
			phase, grant = "fault", nil
		}
		cancel()
		d.mu.Lock()
		d.progress = DesktopProgress{Status: "desktop_status", Phase: phase, RequestID: request, Grant: grant}
		close(d.done)
		d.mu.Unlock()
	}()
}

func (d *desktopFlow) finished() bool {
	select {
	case <-d.done:
		return true
	default:
		return false
	}
}

func (d *desktopFlow) closeJoined() error {
	if d.done == nil {
		return nil
	}
	d.cancel()
	<-d.done
	if d.prepared != nil {
		return d.prepared.Stop(context.Background())
	}
	return nil
}

// workFor is called before the first grant-clock/provision request. Both the
// complete grant and successful consent must belong to the same finished flow.
func (d *desktopFlow) workFor(scope invite.DesktopPermission, grant invite.Grant) SessionWork {
	if !d.finished() {
		return nil
	}
	progress := d.snapshot()
	if progress.Phase != "approved" || progress.Grant == nil || *progress.Grant != grant {
		return nil
	}
	product := d.product
	product.Prepared = d.prepared
	return DesktopProductSession(scope, product)
}

type desktopDeviceRequest func(context.Context, invite.Body) (invite.Result, error)

func (d *desktopFlow) prepare(ctx context.Context, gate *authorizationGate, client *invite.NetworkClient) (string, string, *invite.Grant) {
	scope := invite.DesktopPermission(gate.policy.Permission())
	prepared, err := clientruntime.PrepareDesktop(ctx, scope, gate.self, d.target, d.product.Spec)
	d.prepared = prepared // retain partial ownership even on failure
	if err != nil {
		return "fault", "", nil
	}
	call := func(life context.Context, body invite.Body) (invite.Result, error) {
		bounded, cancel := context.WithTimeout(life, 15*time.Second)
		defer cancel()
		value, err := client.DeviceRequest(bounded, body)
		if err != nil || bounded.Err() != nil || !ValidDesktopDeviceResult(scope, body, value) ||
			(value.Grant != nil && !gate.validGrant(*value.Grant, time.Unix(value.Grant.Claims.IssuedAt, 0))) {
			return invite.Result{}, trustprofile.ErrTrust
		}
		return value, nil
	}
	reg, err := call(ctx, invite.Body{Action: "register"})
	if err != nil || reg.DeviceID != gate.self {
		return "fault", "", nil
	}
	if d.target {
		return d.offer(ctx, gate, call, scope, reg.DeviceCode)
	}
	return d.connect(ctx, gate, call)
}

func (d *desktopFlow) offer(ctx context.Context, gate *authorizationGate, call desktopDeviceRequest, scope invite.DesktopPermission, code string) (string, string, *invite.Grant) {
	// The authority sets its own 120s invitation expiry. This local cap starts
	// before creation, so it cannot extend the invitation or the human wait.
	life, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	inv, err := call(life, invite.Body{Action: "invite", Permission: scope})
	if err != nil || d.prepared.ShowInvite(life, code, inv.Code) != nil {
		return "fault", "", nil
	}
	incoming, phase := d.poll(life, call, invite.Body{Action: "incoming"})
	if phase != "" {
		return phase, "", nil
	}
	request := incoming.RequestID
	d.set("waiting", request, nil)
	allow, err := d.prepared.Decide(life, request, incoming.ControllerID)
	if err != nil {
		return "fault", request, nil
	}
	if allow && d.guiPhase(life) != "" {
		return "fault", request, nil
	}
	decision, err := call(life, invite.Body{Action: "approve", RequestID: request, Allow: allow, Permission: scope})
	if err != nil {
		return "fault", request, nil
	}
	// Verify the authority still reports this precise decision before binding.
	status, err := call(life, invite.Body{Action: "status", RequestID: request})
	if err != nil || status.Status != decision.Status {
		return "fault", request, nil
	}
	if !allow {
		return "denied", request, nil
	}
	if status.Grant == nil || decision.Grant == nil || *status.Grant != *decision.Grant {
		return "fault", request, nil
	}
	return d.bind(gate, request, status.Grant)
}

func (d *desktopFlow) connect(ctx context.Context, gate *authorizationGate, call desktopDeviceRequest) (string, string, *invite.Grant) {
	intent, err := d.prepared.ConnectIntent(ctx)
	if err != nil {
		return "fault", "", nil
	}
	if intent == nil {
		return "stopped", "", nil
	}
	pending, err := call(ctx, intent.Claim())
	if err != nil {
		return "fault", "", nil
	}
	request := pending.RequestID
	d.set("waiting", request, nil)
	life, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	allow, err := d.prepared.Decide(life, request, gate.self)
	if err != nil {
		return "fault", request, nil
	}
	if !allow {
		return "declined", request, nil // a controller cannot deny for the target
	}
	status, phase := d.poll(life, call, invite.Body{Action: "status", RequestID: request})
	if phase != "" {
		return phase, request, nil
	}
	if status.Status != "approved" {
		return status.Status, request, nil
	}
	return d.bind(gate, request, status.Grant)
}

func (d *desktopFlow) bind(gate *authorizationGate, request string, grant *invite.Grant) (string, string, *invite.Grant) {
	if grant == nil || d.prepared.ObserveApproved(gate.issuer, request, *grant) != nil {
		return "fault", request, nil
	}
	return "approved", request, grant
}

func (d *desktopFlow) guiPhase(ctx context.Context) string {
	state, err := d.prepared.State(ctx)
	if err != nil || state.Failed || state.Ended {
		return "fault"
	}
	if state.StopRequested {
		return "stopped"
	}
	return ""
}

func (d *desktopFlow) poll(ctx context.Context, call desktopDeviceRequest, body invite.Body) (invite.Result, string) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		if phase := d.guiPhase(ctx); phase != "" {
			return invite.Result{}, phase
		}
		reply, err := call(ctx, body)
		if err != nil {
			return invite.Result{}, "fault"
		}
		if reply.Status != "idle" && !(body.Action == "status" && reply.Status == "pending") {
			return reply, ""
		}
		select {
		case <-ctx.Done():
			return invite.Result{}, "fault"
		case <-tick.C:
		}
	}
}

func (d *desktopFlow) control(ctx context.Context, request Request, gate *authorizationGate, client *invite.NetworkClient, enrolled *enrollment, output func(any) error) error {
	switch request.Op {
	case "desktop_begin":
		if d.done != nil {
			return output(map[string]string{"status": "request_rejected"})
		}
		d.begin(ctx, gate, client)
		return output(map[string]string{"status": "desktop_pending"})
	case "desktop_status":
		return output(d.snapshot())
	case "desktop_arm_stop":
		if d.stopArmed || !d.armStop(ctx, gate, enrolled) {
			return output(map[string]string{"status": "request_rejected"})
		}
		d.stopArmed = true
		return output(map[string]string{"status": "desktop_stop_armed"})
	case "desktop_state":
		if d.finished() && d.prepared != nil {
			if state, err := d.prepared.State(ctx); err == nil {
				return output(state)
			}
		}
		return output(map[string]string{"status": "product_unavailable"})
	}
	return trustprofile.ErrTrust
}

// Arm only an active session. The controller proves the local stop intent;
// the target must still be serving when it accepts the bounded expectation.
func (d *desktopFlow) armStop(ctx context.Context, gate *authorizationGate, enrolled *enrollment) bool {
	if !d.finished() || d.prepared == nil || enrolled.session == nil ||
		enrolled.watch == nil || enrolled.watch.Faulted() ||
		gate.clock == nil || !gate.validGrant(gate.grant, gate.clock()) {
		return false
	}
	select {
	case <-enrolled.session.Done():
		return false
	case <-enrolled.watch.Done():
		return false
	default:
	}
	state, err := d.prepared.State(ctx)
	wantLocalStop := !d.target
	if err != nil || state.Frames == 0 || state.Failed || state.Ended || state.StopRequested != wantLocalStop {
		return false
	}
	if d.target {
		return d.prepared.ArmTargetStop(ctx) == nil
	}
	return true
}
