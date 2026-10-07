"""Actual Swift/C# framed TLS through helper-owned signed HTTPS gates.

No parent refresh thread/state relay; private test HMAC delivery is not the
final product secret provisioning scheme. No privileged fixture operations.
"""
import base64
import hashlib
import os
from pathlib import Path
import secrets
import subprocess
import time

def counted(value):
    return type(value.get("pongs")) is int and value["pongs"]>=0

def negative_ok(mode, before, after, client):
    if not all(counted(v) for v in (before,after,client)) or client.get("ended") is not True or client.get("pongs")!=0 or after.get("pongs")!=before.get("pongs"):
        return False
    if mode=="pin": return client.get("pin_rejected") is True
    if mode=="key": return client.get("result")=="authentication_rejected"
    if mode=="preauth": return after.get("preauth_rejected",0)>before.get("preauth_rejected",0)
    return False

def closed_ok(before_client,after_client,before_product,after_product):
    return (all(counted(v) for v in (before_client,after_client,before_product,after_product))
            and after_client.get("ended") is True and after_client.get("result")=="connection_closed"
            and before_client.get("pongs")==after_client.get("pongs")
            and before_product.get("pongs")==after_product.get("pongs"))

def wait(check,label,t,timeout=20):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        if check(): t.check(True,label); return
        time.sleep(.2)
    raise RuntimeError(label+"_deadline")

def prepare(t,work):
    # Build before invitation/grant so the short grant isn't spent compiling.
    archive=t.ARTIFACTS/("split-tls-source-"+secrets.token_hex(8)+".tar.gz")
    try:
        env=dict(os.environ,COPYFILE_DISABLE="1")
        packed=subprocess.run(["tar","--format=ustar","--no-xattrs","--no-acls","--exclude=bin","--exclude=obj","--exclude=._*","-czf",str(archive),
            "windows/RemoteAgent/Directory.Build.props","windows/RemoteAgent/src/RemoteProtocol","windows/RemoteAgent/tools/TlsBridgeProbe",
            "windows/RemoteAgent/tests/RemoteProtocol.Tests","protocol/testdata"],cwd=t.HERE.parents[1],env=env,capture_output=True,timeout=15)
        t.check(packed.returncode==0,"split_actual_product_sources_packed")
        uploaded=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(archive),
            "jarvis@100.73.4.118:"+work.replace("\\","/")+"/product-source.tar.gz"],capture_output=True,timeout=60)
        t.check(uploaded.returncode==0,"split_actual_product_sources_uploaded")
        actual=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\product-source.tar.gz').Hash")
        t.check(actual.lower()==hashlib.sha256(archive.read_bytes()).hexdigest(),"split_product_archive_hash_verified")
        script=(f"$ErrorActionPreference='Stop'; tar -xzf '{work}\\product-source.tar.gz' -C '{work}'; if ($LASTEXITCODE -ne 0) {{throw 'unpack_failed'}}; "
                f"dotnet run --project '{work}\\windows\\RemoteAgent\\tests\\RemoteProtocol.Tests' -c Release -- --bridge-validation; "
                "if ($LASTEXITCODE -ne 0) {throw 'tests_failed'}; "
                f"dotnet publish '{work}\\windows\\RemoteAgent\\tools\\TlsBridgeProbe' -c Release -r win-x64 --self-contained true -o '{work}\\product-publish'; "
                "if ($LASTEXITCODE -ne 0) {throw 'publish_failed'}")
        encoded=base64.b64encode(script.encode("utf-16le")).decode()
        result=subprocess.run(t.WIN+["powershell.exe -NoProfile -NonInteractive -EncodedCommand "+encoded],capture_output=True,timeout=180)
        output=result.stdout.decode(errors="replace")
        t.check(result.returncode==0 and "65/65 tests passed" in output and "FAIL " not in output
                and "SKIP Windows credential persistence" in output,"split_actual_windows_protocol_tests_65_dpapi_not_validated")
        return t.WIN+[f'cmd.exe /d /c "{work}\\product-publish\\TlsBridgeProbe.exe"']
    finally:
        if archive.exists(): archive.unlink()

class ActiveTLS:
    def __init__(self,t,client,product,server,peer,server_baseline,product_baseline):
        self.t,self.client,self.product,self.server,self.peer=t,client,product,server,peer
        self.server_baseline,self.product_baseline=server_baseline,product_baseline

    def revoked(self):
        self.closed("revocation")

    def closed(self,reason):
        # This is checked before the coordinator waits for node reclamation.
        # It does not claim old ACL retention; that requires a fault slice.
        wait(lambda: all(h.request("count")["managed_done"] for h in (self.server,self.peer)),"split_tls_autonomous_watches_denied_and_joined",self.t)
        wait(lambda:self.client.request("count")["ended"] and self.server.request("count")["stream_closed"]==self.server_baseline+1
             and self.product.request("count")["sessions_closed"]==self.product_baseline+1,"split_actual_product_tls_"+reason+"_closed",self.t)
        before_client=self.client.request("count"); before_product=self.product.request("count")
        time.sleep(.5)
        self.t.check(closed_ok(before_client,self.client.request("count"),before_product,self.product.request("count")),"split_"+reason+"_tls_no_more_pongs_not_client_timeout")

def run(t,Pipe,ready,apis,node,grant,command,helpers,reuse=None):
    if reuse is None:
        product=Pipe(command); helpers.append(product)
        public=product.read()
    else: product,public=reuse.product,reuse.public
    t.check(public.get("status")=="ready","split_actual_csharp_tls_agent_ready")
    server,peer=node["restart_managed"]()
    state=apis[1].request("network_state")
    t.check(state["passed"] and state["network_state"]["claims"]["active"] is True
            and state["network_state"]["claims"]["binding"]==node["binding"],"split_tls_actual_binding_active")
    for helper,role in ((server,"target"),(peer,"controller")):
        t.check(helper.request("authorize",Role=role,Issuer=ready["issuer"],Target=ready["target"],Controller=ready["controller"],State=state["network_state"],Bridge=True)["passed"],"split_tls_helper_owned_https_gate_enabled")
        t.check(helper.request("count")["managed_active"],"split_tls_autonomous_watch_active")
        t.check(not helper.request("state",State=state["network_state"])["passed"],"split_tls_parent_state_cannot_authorize_gate")
    if reuse is None:
        challenge=secrets.token_hex(32)
        certificate=product.request("certificate_proof",GrantID=grant["claims"]["id"],Target=grant["claims"]["target"],Challenge=challenge)
        published=apis[0].request("publish_tls",Challenge=challenge,Certificate=certificate["Certificate"],CertificateProof=certificate["CertificateProof"])
        t.check(published["passed"],"split_os_target_publishes_real_tls_key_possession")
    port=peer.request("bridge",Role="controller")["port"]
    executable=t.HERE.parents[1]/"macos/RemoteController/.build/release/TLSBridgeProbe"
    def launch(**changes):
        descriptor=apis[1].request("tls_identity",Binding=node["binding"])
        t.check(descriptor["passed"] and descriptor["tls_identity"]["claims"]["fingerprint"]==public["Fingerprint"],"split_controller_fresh_issuer_tls_descriptor_matches_product")
        cfg={"Port":port,"Key":public["Key"],"Fingerprint":descriptor["tls_identity"]["claims"]["fingerprint"],"Target":grant["claims"]["target"]}
        cfg.update(changes)
        client=Pipe([str(executable)],cfg); helpers.append(client)
        t.check(client.read()["status"]=="ready","split_actual_swift_tls_controller_ready")
        return client
    negatives=(("pin",{"Fingerprint":"00"*32}),("key",{"Key":base64.b64encode(bytes(32)).decode()}),("preauth",{"Preauth":True})) if reuse is None else ()
    for mode,changes in negatives:
        before=product.request("count")
        negative=launch(**changes)
        wait(lambda:negative.request("count")["ended"],"split_actual_tls_negative_ended",t,8)
        t.check(negative_ok(mode,before,product.request("count"),negative.request("count")),"split_actual_tls_"+mode+"_rejected_zero_protected_payload")
        negative.stop(); helpers.remove(negative)
    if reuse is None: wait(lambda:server.request("count")["stream_closed"]>=3 and product.request("count")["sessions_closed"]>=3,"split_negative_tunnels_closed",t)
    baseline=server.request("count")["stream_closed"]; product_baseline=product.request("count")["sessions_closed"]
    client=launch()
    wait(lambda:client.request("count")["pongs"]>=3,"split_actual_tls_hmac_framed_pongs",t,8)
    before=client.request("count")["pongs"]
    time.sleep(5)
    t.check(client.request("count")["pongs"]>before+10 and not client.request("count")["ended"]
            and server.request("count")["stream_closed"]==baseline,"split_same_tls_connection_beyond_initial_lease_autonomous_refresh")
    active=ActiveTLS(t,client,product,server,peer,baseline,product_baseline)
    active.public=public
    return active

def fault_ack(value,status):
    expected={"status","old_policy_retained","https_503"} if status=="frozen" else {"status","explicit_restart","same_verifier"}
    return set(value)==expected and value.get("status")==status and all(value[k] is True for k in expected-{"status"})

def fault_round(t,Pipe,ready,apis,node,grant,command,helpers,coordinator,active):
    b=node["binding"]
    expected={"acls":[{"action":"accept","src":[b["controller_ip"]],"dst":[b["target_ip"]+":47476"]}]}
    t.check(t.admin("policy","get")==expected,"split_fault_exact_live_acl_before_pause")
    t.check(fault_ack(coordinator.request("freeze"),"frozen"),"split_isolated_authority_paused_https503_old_acl_retained")
    t.check(t.admin("policy","get")==expected,"split_old_acl_still_present_during_tls_fault_close")
    active.closed("authority_fault")
    t.check(t.admin("policy","get")==expected,"split_old_acl_remains_after_actual_tls_closed")
    before=active.client.request("count");product_before=active.product.request("count")
    t.check(fault_ack(coordinator.request("recover"),"recovered"),"split_explicit_same_verifier_authority_restart")
    wait(lambda:t.admin("policy","get")==expected,"split_recovery_approved_minimum_policy_restored",t)
    fresh=apis[1].request("network_state")
    t.check(fresh["passed"] and fresh["network_state"]["claims"]["active"] is True and fresh["network_state"]["claims"]["binding"]==b,"split_recovery_fresh_signed_same_binding")
    for h in (active.server,active.peer):
        counts=h.request("count")
        t.check(counts["managed_done"] and not counts["managed_active"],"split_old_managed_gate_not_reopened_by_recovery")
        t.check(not h.request("state",State=fresh["network_state"])["passed"],"split_fresh_parent_state_cannot_reopen_closed_gate")
    time.sleep(.5)
    t.check(closed_ok(before,active.client.request("count"),product_before,active.product.request("count")),"split_old_tls_not_reopened_after_authority_recovery")
    active.client.stop();helpers.remove(active.client)
    # Explicit new helpers, clock/watch/gates and new TLS; never revive old ones.
    return run(t,Pipe,ready,apis,node,grant,command,helpers,reuse=active)
