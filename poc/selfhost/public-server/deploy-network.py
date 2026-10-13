"""Explicit approved maintenance coordinator; no implicit public activation."""
import argparse,base64,hashlib,importlib.util,json,re,subprocess
from pathlib import Path

spec=importlib.util.spec_from_file_location("transport",Path(__file__).with_name("daemon-network.py"))
d=importlib.util.module_from_spec(spec);spec.loader.exec_module(d);t=d.t

def main():
    p=argparse.ArgumentParser()
    p.add_argument("--maintenance-approved",action="store_true")
    p.add_argument("--stage",required=True)
    p.add_argument("mode",choices=["stage","prepare","activate","fault","resume","rollback","refresh-probe"])
    a=p.parse_args()
    t.check(a.maintenance_approved and re.fullmatch(r"/run/remoteapp-network-maint-[0-9a-f]{16}",a.stage) is not None,"explicit_maintenance_authority_and_scope_required")
    if a.mode=="stage":
        r=subprocess.run(t.SERVER+["test ! -e "+a.stage+" && install -d -m 700 "+a.stage],capture_output=True,timeout=15)
        t.check(r.returncode==0,"fresh_root_private_maintenance_stage_created")
        sources={
            "authority-daemon":t.ARTIFACTS/"authority-daemon-network",
            "device-frontend":t.ARTIFACTS/"device-frontend-direct-r19",
            "verifier-owner":t.ARTIFACTS/"verifier-owner-r22",
            "network-deployment-check":t.ARTIFACTS/"network-deployment-check-r22",
            "authority.candidate":Path(__file__).with_name("remoteapp-authority-network.service.candidate"),
        }
        for name,source in sources.items(): d.upload(source,a.stage+"/"+name)
        d.upload(Path(__file__).with_name("network-maintenance.py"),a.stage+"/network-maintenance.py")
        manifest={"version":1,"sha256":{name:hashlib.sha256(source.read_bytes()).hexdigest() for name,source in sources.items()}}
        payload=base64.b64encode(json.dumps(manifest,sort_keys=True).encode()).decode()
        code="import base64,os; os.umask(0o077); f=open('"+a.stage+"/manifest.json','wb'); f.write(base64.b64decode('"+payload+"')); f.flush(); os.fsync(f.fileno()); f.close()"
        import shlex
        r=subprocess.run(t.SERVER+["python -c "+shlex.quote(code)],capture_output=True,timeout=15)
        t.check(r.returncode==0,"maintenance_manifest_staged_no_service_change")
        return
    if a.mode=="refresh-probe":
        source=t.ARTIFACTS/"network-deployment-check-r22"
        d.upload(source,a.stage+"/network-deployment-check")
        d.upload(Path(__file__).with_name("network-maintenance.py"),a.stage+"/network-maintenance.py")
        code="import json,os,tempfile; path='"+a.stage+"/manifest.json'; m=json.load(open(path)); m['sha256']['network-deployment-check']='"+hashlib.sha256(source.read_bytes()).hexdigest()+"'; fd,tmp=tempfile.mkstemp(dir='"+a.stage+"'); f=os.fdopen(fd,'w'); json.dump(m,f,sort_keys=True); f.flush(); os.fsync(f.fileno()); f.close(); os.chmod(tmp,0o600); os.rename(tmp,path)"
        import shlex
        r=subprocess.run(t.SERVER+["python -c "+shlex.quote(code)],capture_output=True,timeout=15)
        t.check(r.returncode==0,"pinned_probe_manifest_updated_services_untouched")
    r=subprocess.run(t.SERVER+["python "+a.stage+"/network-maintenance.py "+a.stage+" "+a.mode],capture_output=True,timeout=240)
    if r.returncode:
        for line in r.stdout.decode().splitlines():
            if re.fullmatch(r"DIAGNOSTIC maintenance_[a-z_]+",line): print(line,flush=True)
    expected=("PASS prerelease_network_maintenance_"+a.mode).encode()
    t.check(r.returncode==0 and expected in r.stdout.splitlines(),"maintenance_"+a.mode+"_completed")
    print(expected.decode(),flush=True)

if __name__=="__main__":
    try: main()
    except Exception:
        print("FAIL maintenance_coordinator_preserve_checkpoint_for_review",flush=True)
        raise SystemExit(1)
