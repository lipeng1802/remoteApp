"""Private nodeproof phase. Device operations stay on signed HTTPS clients."""
import hashlib
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

def upload_helper(t, work):
    """Finish public transfer and verification BEFORE issuing a short grant."""
    binary=t.ARTIFACTS/"enrollment-helper-win.exe"
    result=subprocess.run(["scp","-O","-C","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(binary),"jarvis@100.73.4.118:"+work.replace("\\","/")+"/enrollment-helper.exe"],capture_output=True,timeout=180)
    t.check(result.returncode==0,"split_node_helper_uploaded")
    verify_helper(t, work)


def verify_helper(t, work):
    binary=t.ARTIFACTS/"enrollment-helper-win.exe"
    actual=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\enrollment-helper.exe').Hash")
    t.check(actual.lower()==hashlib.sha256(binary.read_bytes()).hexdigest(),"split_node_helper_binary_verified")


def run(t, Pipe, identities, ready, apis, ports, work, grant, helpers, folders):
    # Recheck before execution; no upload after the grant has been issued.
    verify_helper(t, work)
    # Helper validates its node directory against ../../artifacts/... from cwd.
    win_work=work+"\\poc\\selfhost"
    t.powershell(f"New-Item -ItemType Directory -Path '{win_work}' -Force -ErrorAction Stop | Out-Null")
    win=identities.command(win_work,"..\\..\\enrollment-helper.exe")
    local=[str(t.ARTIFACTS/"enrollment-helper")]
    parent=t.ARTIFACTS/"nodes"; parent.mkdir(mode=0o700,exist_ok=True)
    folder=Path(tempfile.mkdtemp(prefix="split-node-",dir=parent)); folders.append(folder)
    win_dir="../../artifacts/connection-poc/nodes/target"
    def prepare(command,directory,windows):
        cfg={"Op":"prepare","StateDir":str(directory),"GrantID":grant["claims"]["id"]}
        cfg.update(identities.config("native-target" if windows else "native-controller",windows))
        p=Pipe(command,cfg)
        try:
            reply=p.read(); p.proc.stdin.close(); p.proc.wait(timeout=8)
            t.check(p.proc.returncode==0 and reply.get("status")=="prepared","split_offline_key_and_os_signed_intent")
            return reply
        finally: p.emergency()
    target=prepare(win,win_dir,True); controller=prepare(local,folder,False)
    t.check(len(t.admin("nodes","list") or [])==1,"split_prepare_no_client_network_registration")
    tickets=[]
    for api, prepared in zip(apis,(target,controller)):
        issued=api.request("enroll",Intent=prepared["intent"])
        t.check(issued["passed"],"split_signed_https_client_enrollment")
        t.check(not api.request("enroll",Intent=prepared["intent"])["passed"],"split_enrollment_intent_single_use")
        tickets.append(issued)
    token=secrets.token_hex(32)  # Private fixed-payload test only, not product HMAC distribution.
    def join(command,directory,prepared,ticket,windows,wrong=False,managed=False):
        cfg={"Op":"join","StateDir":str(directory),"AuthKey":ticket["credential"]["Secret"],
             "NodeKey":prepared["node_key"],"Verifier":ready["verifier"],"Token":token,"GrantID":grant["claims"]["id"]}
        cfg.update(identities.config("native-target" if windows or wrong else "native-controller",windows))
        if managed:
            cfg.update(Token="",Endpoint=f"https://127.0.0.1:{ports[0] if windows else ports[1]}",
                       Certificate=ready["api_certificate"],Issuer=ready["issuer"])
        p=Pipe(command,cfg); helpers.append(p)
        registered=p.read()
        t.check(registered.get("status")=="registered" and registered["node_key"]==prepared["node_key"],"split_join_key_matches_prepared")
        t.check(p.read().get("status")=="ready","split_node_listeners_ready")
        return p,next(ip for ip in registered["addresses"] if ":" not in ip)
    server,target_ip=join(win,win_dir,target,tickets[0],True)
    peer,controller_ip=join(local,folder,controller,tickets[1],False,True)
    t.check(not peer.request("probe",Peer=target_ip)["passed"],"split_unbound_zero_policy_denies_payload")
    t.check(apis[0].request("bind",Ticket=tickets[0]["ticket"])["passed"],"split_windows_real_node_and_os_possession")
    t.check(not apis[1].request("bind",Ticket=tickets[1]["ticket"])["passed"],"split_wrong_mac_os_identity_bind_rejected")
    counts=peer.request("count")
    t.check(counts["challenges"]>=1 and counts["signed"]==0,"split_wrong_os_identity_received_real_unsigned_challenge")
    t.check(t.admin("policy","get")=={"acls":[]},"split_failed_proof_no_payload_acl")
    peer.stop(); helpers.remove(peer)
    peer,again=join(local,folder,controller,tickets[1],False)
    t.check(again==controller_ip,"split_mac_restart_same_node_identity")
    t.check(apis[1].request("bind",Ticket=tickets[1]["ticket"])["passed"],"split_mac_real_node_and_os_possession")
    expected={"acls":[{"action":"accept","src":[controller_ip],"dst":[target_ip+":47476"]}]}
    end=time.monotonic()+12
    while t.admin("policy","get")!=expected:
        if time.monotonic()>end: raise RuntimeError("split_projection_timeout")
        time.sleep(.25)
    binding={"target_node":target["node_key"],"controller_node":controller["node_key"],"target_ip":target_ip,"controller_ip":controller_ip}
    for api in apis:
        reply=api.request("network_state")
        claims=reply.get("network_state",{}).get("claims",{})
        t.check(reply["passed"] and claims.get("active") is True and claims.get("binding")==binding,"split_signed_binding_matches_actual_pair")
    t.check(peer.request("probe",Peer=target_ip)["passed"],"split_forward_actual_fixed_payload_allowed")
    reverse=server.request("probe",Peer=controller_ip)
    t.check(not reverse["passed"] and reverse.get("reason") in ("route_denied","dial_denied") and peer.request("count")["value"]==0,"split_reverse_payload_denied")
    def restart_managed():
        nonlocal peer,server
        peer.stop(); helpers.remove(peer)
        server.stop(); helpers.remove(server)
        managed_server,ip=join(win,win_dir,target,tickets[0],True,managed=True)
        managed_peer,cip=join(local,folder,controller,tickets[1],False,managed=True)
        t.check(ip==target_ip and cip==controller_ip,"split_managed_restart_same_node_binding")
        t.check(not managed_peer.request("probe",Peer=target_ip)["passed"],"split_managed_has_no_token_fallback")
        server,peer=managed_server,managed_peer
        return managed_server,managed_peer
    from compiled_coordination import BoundHandoff, runtime_inputs
    inputs=runtime_inputs(ready,binding,grant,work+"\\artifacts\\connection-poc\\nodes\\target",str(folder),tickets)
    def stop_bound_helpers():
        nonlocal peer,server
        # No new tsnet owner may reopen either directory before actual join.
        # Failed stop leaves its handle/evidence in the parent's cleanup list.
        for helper in (peer,server):
            helper.stop(); helpers.remove(helper)
        peer,server=None,None
    return {"peer":peer,"target_ip":target_ip,"binding":binding,"restart_managed":restart_managed,
            "compiled_handoff":BoundHandoff(inputs,stop_bound_helpers)}

def reclaimed(t, timeout=20):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        if len(t.admin("nodes","list") or [])==1 and t.admin("policy","get")=={"acls":[]}:
            t.check(True,"split_revoked_client_nodes_reclaimed_zero_acl")
            return
        time.sleep(.25)
    raise RuntimeError("split_client_retirement_unconfirmed")
