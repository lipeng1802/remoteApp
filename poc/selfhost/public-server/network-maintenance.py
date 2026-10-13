"""Root-local prerelease maintenance only, Python 2 compatible.

Explicit commands/checkpoints; no auto rollback, no Store snapshot restoration,
no force kill/lock removal, no public listener or business Nginx mutation.
"""
from __future__ import print_function
import errno,hashlib,json,os,pwd,re,select,shutil,signal,stat,subprocess,sys,tempfile,time

AUTH="/etc/systemd/system/remoteapp-authority.service"
FRONT="/etc/systemd/system/remoteapp-frontend.service"
CONTROL="/etc/remoteapp-poc/config.yaml"
STORE="/var/lib/remoteapp-authority/state.json"
VD="/var/lib/remoteapp-verifier"
PD="/var/lib/remoteapp-network-policy"
NC="/etc/remoteapp-authority/network.json"
HS=["/opt/remoteapp-poc/headscale","-c",CONTROL,"-o","json"]
FILES=("authority-daemon","device-frontend","verifier-owner","network-deployment-check","authority.candidate")

def require(ok,label):
    if not ok: raise RuntimeError(label)

def digest(path):
    return hashlib.sha256(open(path,"rb").read()).hexdigest()

def command(args):
    p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    out,unused=p.communicate()
    require(p.returncode==0,"command_failed")
    return out

def system(*args): return command(["systemctl"]+list(args))

def properties(name):
    out=system("show",name,"-p","MainPID","-p","ExecMainPID","-p","ActiveState","-p","ExecMainStatus","-p","Restart")
    return dict(line.split("=",1) for line in out.decode().splitlines())

def gone(pid):
    try: os.kill(pid,0); return False
    except OSError as e: return e.errno==errno.ESRCH

def logged(name,pid,label):
    return label in command(["journalctl","--no-pager","-o","cat","-u",name,"_PID="+str(pid)]).decode().splitlines()

def stopped(name,label,legacy_hash=None):
    p=properties(name);pid=int(p["MainPID"])
    require(pid>1 and p["ActiveState"]=="active","service_not_active")
    system("stop",name)
    after=properties(name)
    receipt=logged(name,pid,label)
    if not receipt and legacy_hash is not None and name=="remoteapp-frontend":
        # Only migration from the inventoried old device-only binary lacks a
        # STOPPED label. Actual PID disappearance, clean systemd status and
        # absent listener are still mandatory. Never applies to new binaries.
        old=open(FRONT,"rb").read()
        listener=command(["ss","-lntH"]).decode().splitlines()
        require(b"--public-network" not in old and b"--private-network" not in old and digest("/opt/remoteapp-prerelease/device-frontend")==legacy_hash,"legacy_binary_not_pinned")
        receipt=after["ExecMainStatus"]=="0" and not any(len(line.split())>3 and line.split()[3].endswith(":8444") for line in listener)
    require(gone(pid) and after["MainPID"]=="0" and receipt,"stop_not_joined")

def ready(name,label):
    for unused in range(450):
        p=properties(name);pid=int(p["MainPID"])
        if p["ActiveState"]=="failed": raise RuntimeError("candidate_failed")
        if pid>1 and p["ActiveState"]=="active" and logged(name,pid,label): return pid
        time.sleep(.1)
    raise RuntimeError("candidate_not_ready")

def atomic(path,data,mode,uid=0,gid=0):
    parent=os.path.dirname(path)
    require(os.path.realpath(parent)==parent and not os.path.islink(path),"unsafe_write_target")
    fd,tmp=tempfile.mkstemp(prefix=".remoteapp-maint-",dir=parent)
    try:
        with os.fdopen(fd,"wb") as f: f.write(data);f.flush();os.fsync(f.fileno())
        os.chmod(tmp,mode);os.chown(tmp,uid,gid);os.rename(tmp,path)
        fd=os.open(parent,os.O_RDONLY)
        try: os.fsync(fd)
        finally: os.close(fd)
    finally:
        if os.path.exists(tmp): os.unlink(tmp)

def save(backup,state): atomic(backup+"/checkpoint.json",json.dumps(state,sort_keys=True).encode(),0o600)

def empty_control():
    for kind in ("nodes","users","preauthkeys"):
        require(not json.loads(command(HS+[kind,"list"]) or b"null"),"control_not_empty")

def zero_policy(): require(json.loads(command(HS+["policy","get"]))=={"acls":[]},"policy_not_zero")

def no_locks():
    for path in ("/var/lib/remoteapp-authority/writer.lock",VD+"/owner.lock"):
        require(not os.path.lexists(path),"owner_lock_retained")

def identity():
    j=json.load(open(VD+"/journal.json"))
    require(j["Phase"]=="ready","verifier_not_ready")
    return hashlib.sha256((j["Record"]["NodeKey"]+"|"+j["Observation"]["IP"]).encode()).hexdigest()

def probe(mode):
    out=command(["runuser","-u","remoteapp-pre","--","/opt/remoteapp-prerelease/network-deployment-check",mode])
    require(out==("PASS deployed_network_"+mode+"_kernel_uid_verified_tls_negative_requests\n").encode(),"probe_failed")

def stage_valid(stage):
    return bool(re.match(r"^/run/remoteapp-network-maint-[0-9a-f]{16}\Z",stage)) and os.path.realpath(stage)==stage

def main(stage,mode):
    os.umask(0o077)
    require(os.geteuid()==0 and stage_valid(stage),"invalid_stage")
    i=os.lstat(stage);require(i.st_uid==0 and stat.S_IMODE(i.st_mode)==0o700,"unsafe_stage_permissions")
    manifest=json.load(open(stage+"/manifest.json"))
    require(manifest.get("version")==1 and set(manifest.get("sha256",{}))==set(FILES),"invalid_manifest")
    for name in FILES:
        path=stage+"/"+name;i=os.lstat(path)
        require(stat.S_ISREG(i.st_mode) and i.st_uid==0 and digest(path)==manifest["sha256"][name],"artifact_mismatch")
    backup="/var/lib/remoteapp-network-maintenance/"+os.path.basename(stage)
    if mode=="prepare":
        require(not os.path.exists(backup) and all(not os.path.lexists(p) for p in (VD,PD,NC)),"existing_network_material")
        empty_control()
        for n in ("remoteapp-authority","remoteapp-frontend","remoteapp-poc"): require(properties(n)["ActiveState"]=="active","existing_service_unhealthy")
        raw=open(CONTROL,"rb").read();require(raw.count(b"  mode: file\n")==1,"unexpected_policy_mode")
        oldauth=open(AUTH,"rb").read();oldfront=open(FRONT,"rb").read()
        require(b"--network-config" not in oldauth and b"--public-network" not in oldfront and b"127.0.0.1:8444" in oldfront,"unexpected_old_unit")
        require(not json.load(open(STORE)).get("Provisions"),"existing_provision_work")
        if not os.path.exists(os.path.dirname(backup)): os.mkdir(os.path.dirname(backup),0o700)
        os.mkdir(backup,0o700)
        targets=[AUTH,FRONT,CONTROL,"/etc/remoteapp-poc/policy.json","/opt/remoteapp-prerelease/authority-daemon","/opt/remoteapp-prerelease/device-frontend"]
        state={"phase":"backup_ready","files":{}}
        for index,path in enumerate(targets):
            require(os.path.realpath(path)==path,"symlink_backup_target")
            i=os.stat(path);name="original-"+str(index)
            shutil.copyfile(path,backup+"/"+name);os.chmod(backup+"/"+name,0o600)
            state["files"][path]={"name":name,"hash":digest(path),"mode":stat.S_IMODE(i.st_mode),"uid":i.st_uid,"gid":i.st_gid}
        state["nginx_baseline"]=digest("/opt/remoteapp-prerelease/nginx-baseline.sha256")
        save(backup,state)
        stopped("remoteapp-frontend","STOPPED frontend",state["files"]["/opt/remoteapp-prerelease/device-frontend"]["hash"])
        stopped("remoteapp-authority","STOPPED authority")
        no_locks()
        shutil.copyfile(STORE,backup+"/store-for-audit-only.json");os.chmod(backup+"/store-for-audit-only.json",0o600)
        state["store_hash"]=digest(STORE);state["phase"]="quiesced";save(backup,state)
        meta=state["files"][CONTROL]
        atomic(CONTROL,raw.replace(b"  mode: file\n",b"  mode: database\n"),meta["mode"],meta["uid"],meta["gid"])
        system("restart","remoteapp-poc")
        atomic(backup+"/deny.json",b'{"acls":[]}',0o600)
        command(HS+["policy","set","--file",backup+"/deny.json"]);zero_policy()
        state["phase"]="database_zero";save(backup,state)
        p=subprocess.Popen([stage+"/verifier-owner","create",VD],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        state["creator_pid"]=p.pid;state["phase"]="verifier_creating";save(backup,state)
        watch,unused,unused2=select.select([p.stdout],[],[],45)
        require(watch and p.stdout.readline()==b"READY persistent verifier only; no payload ACL or API activation\n","creator_not_ready")
        p.send_signal(signal.SIGTERM)
        for unused in range(450):
            if p.poll() is not None: break
            time.sleep(.1)
        require(p.poll()==0 and p.stdout.read()==b"STOPPED verifier; identity retained for explicit resume\n","creator_not_joined")
        no_locks();os.mkdir(PD,0o700)
        atomic(NC,json.dumps({"version":1,"verifier_directory":VD,"policy_directory":PD},separators=(",",":")).encode(),0o600)
        state["verifier_identity"]=identity();state["phase"]="prepared";save(backup,state)
    else:
        state=json.load(open(backup+"/checkpoint.json"))
        if mode=="activate":
            require(state["phase"]=="prepared","wrong_checkpoint")
            no_locks()
            a=json.load(open("/var/lib/remoteapp-prerelease/client-public/installation-anchor.json"))
            require(re.match(r"^[0-9a-f]{64}$",a["release_root"]) and a["service"]=="remoteapp-prerelease" and a["minimum_epoch"]==1,"unexpected_anchor")
            user=pwd.getpwnam("remoteapp-pre")
            unit=open(stage+"/authority.candidate","rb").read().decode()
            for key,value in (("@RELEASE_ROOT@",a["release_root"]),("@FRONTEND_UID@",str(user.pw_uid)),("@FRONTEND_GID@",str(user.pw_gid))): unit=unit.replace(key,value)
            require("@" not in unit and "Restart=no" in unit,"invalid_candidate")
            front=open(backup+"/"+state["files"][FRONT]["name"],"rb").read().decode()
            require(front.count("Requires=remoteapp-authority.service")==1 and front.count(" 0\n")==1,"unexpected_frontend_template")
            front=front.replace("Requires=remoteapp-authority.service","Wants=remoteapp-authority.service").replace(" 0\n"," 0 --public-network\n")
            state["phase"]="installing";save(backup,state)
            state["candidate_hashes"]={AUTH:hashlib.sha256(unit.encode()).hexdigest(),FRONT:hashlib.sha256(front.encode()).hexdigest(),CONTROL:hashlib.sha256(open(backup+"/"+state["files"][CONTROL]["name"],"rb").read().replace(b"  mode: file\n",b"  mode: database\n")).hexdigest()}
            for name in ("authority-daemon","device-frontend"): state["candidate_hashes"]["/opt/remoteapp-prerelease/"+name]=manifest["sha256"][name]
            save(backup,state)
            for name in FILES[:-1]: atomic("/opt/remoteapp-prerelease/"+name,open(stage+"/"+name,"rb").read(),0o755)
            atomic(AUTH,unit.encode(),0o644);atomic(FRONT,front.encode(),0o644)
            system("daemon-reload");system("start","remoteapp-authority")
            ready("remoteapp-authority","READY sole authority; supervised network enabled on private broker")
            system("start","remoteapp-frontend")
            ready("remoteapp-frontend","READY unprivileged frontend; explicit direct signed network mode")
            probe("active");zero_policy()
            require(digest(STORE)==state["store_hash"] and identity()==state["verifier_identity"],"identity_or_store_changed")
            state["phase"]="active";save(backup,state)
        elif mode=="refresh-probe":
            require(state["phase"]=="verified_active","wrong_checkpoint")
            atomic("/opt/remoteapp-prerelease/network-deployment-check",open(stage+"/network-deployment-check","rb").read(),0o755)
            probe("active")
            require(digest(STORE)==state["store_hash"],"store_changed")
            operator=backup+"/operator"
            if os.path.isdir(operator):
                for name in ("network-deployment-check","manifest.json","network-maintenance.py"):
                    atomic(operator+"/"+name,open(stage+"/"+name,"rb").read(),0o755 if name=="network-deployment-check" else 0o600)
        elif mode=="fault":
            require(state["phase"]=="active","wrong_checkpoint")
            pid=int(properties("remoteapp-authority")["MainPID"]);require(pid>1,"authority_missing")
            state["phase"]="fault_pending";save(backup,state)
            try:
                system("stop","remoteapp-poc")
                for unused in range(450):
                    if properties("remoteapp-authority")["MainPID"]=="0": break
                    time.sleep(.1)
                require(gone(pid) and logged("remoteapp-authority",pid,"FAIL authority unavailable") and properties("remoteapp-authority")["ExecMainStatus"]!="0","fault_not_closed")
                no_locks();probe("unavailable")
            finally: system("start","remoteapp-poc")
            time.sleep(2)
            require(properties("remoteapp-authority")["MainPID"]=="0" and not os.path.exists("/run/remoteapp-authority/authority.sock"),"automatic_recovery")
            probe("unavailable");state["phase"]="fault_closed";save(backup,state)
        elif mode=="resume":
            require(state["phase"]=="fault_closed","wrong_checkpoint")
            no_locks();system("start","remoteapp-authority")
            ready("remoteapp-authority","READY sole authority; supervised network enabled on private broker")
            probe("active");zero_policy()
            require(digest(STORE)==state["store_hash"] and identity()==state["verifier_identity"],"identity_or_store_changed")
            state["phase"]="verified_active";save(backup,state)
        elif mode=="rollback":
            require(state["phase"] in ("backup_ready","quiesced","database_zero","verifier_creating","prepared","installing","active","fault_pending","fault_closed","verified_active"),"rollback_not_reviewable")
            for name,label in (("remoteapp-frontend","STOPPED frontend"),("remoteapp-authority","STOPPED authority")):
                p=properties(name)
                if p["ActiveState"]=="active": stopped(name,label)
                else:
                    require(p["MainPID"]=="0","owner_still_live")
                    pid=int(p.get("ExecMainPID","0"));require(pid==0 or gone(pid),"old_owner_still_live")
            no_locks()
            require(not state.get("creator_pid") or gone(state["creator_pid"]),"creator_still_live")
            if os.path.lexists(VD):
                require(os.path.isfile(VD+"/journal.json"),"unknown_verifier_outcome")
                out=command([stage+"/verifier-owner","retire",VD])
                require(out==b"RETIRED scoped verifier resources\n","retirement_unconfirmed")
            empty_control()
            for path,meta in state["files"].items():
                allowed=[meta["hash"],state.get("candidate_hashes",{}).get(path)]
                if path==CONTROL: allowed.append(hashlib.sha256(open(backup+"/"+meta["name"],"rb").read().replace(b"  mode: file\n",b"  mode: database\n")).hexdigest())
                require(digest(path) in allowed,"external_change_rollback_refused")
                atomic(path,open(backup+"/"+meta["name"],"rb").read(),meta["mode"],meta["uid"],meta["gid"])
            # Keep retired verifier/config and current Store for audit/recovery.
            # Never restore store-for-audit-only.json or delete locks/journals.
            system("daemon-reload");system("restart","remoteapp-poc")
            system("start","remoteapp-authority");ready("remoteapp-authority","READY sole authority; network disabled")
            system("start","remoteapp-frontend");ready("remoteapp-frontend","READY unprivileged frontend; network disabled")
            state["phase"]="rolled_back";save(backup,state)
        else: raise RuntimeError("invalid_mode")
    command(["sha256sum","--check","/opt/remoteapp-prerelease/nginx-baseline.sha256"])
    print("PASS prerelease_network_maintenance_"+mode)

if __name__=="__main__":
    try:
        require(len(sys.argv)==3,"arguments")
        main(sys.argv[1],sys.argv[2])
    except Exception as e:
        if isinstance(e,RuntimeError) and re.match(r"^[a-z_]+\Z",str(e)):
            print("DIAGNOSTIC maintenance_"+str(e))
        print("FAIL maintenance_paused_state_preserved_no_automatic_rollback")
        sys.exit(1)
