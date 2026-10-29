"""Phase gates for a separately prepared, resident TEST activity pair.

No setup, credentials, grant persistence, implicit teardown or resource deletion.
Adapters/holders are caller-owned. Unknown exercise outcomes retain all owners.
"""
import json
import os
from pathlib import Path

from supervised_session import active_round


class Activity:
    def __init__(self, directory):
        self.directory = Path(directory)
        self.path = self.directory/"activity-state.json"

    def read(self):
        value=json.loads(self.path.read_text())
        if not isinstance(value,dict) or type(value.get("version")) is not int or value["version"]!=1:
            raise RuntimeError("activity_state_rejected")
        return value

    def save(self,value):
        # Serial trusted operator only. Exclusive temporary file and atomic
        # replacement prevent half-written public phase evidence after a crash.
        pending=self.directory/(".activity-state."+str(os.getpid())+".new")
        with pending.open("x") as output:
            json.dump(value,output,sort_keys=True)
            output.flush();os.fsync(output.fileno())
        os.replace(pending,self.path)
        descriptor=os.open(self.directory,os.O_RDONLY)
        try:os.fsync(descriptor)
        finally:os.close(descriptor)

    def exercise(self,target,controller,expiry,episode=active_round):
        state=self.read()
        if state.get("phase")!="owners_ready":
            raise RuntimeError("activity_requires_fresh_ready_owners")
        # Persist uncertainty BEFORE registration/claim/provision can escape.
        # An exception leaves no automatic restart/reapproval/stop permission.
        self.save({"version":1,"phase":"exercise_outcome_unknown"})
        try:
            result=episode(target,controller,profile_expires_at=expiry,quiesce_before_revoke=True)
        except Exception:
            raise RuntimeError("activity_outcome_unknown_preserve_owners_and_tunnels") from None
        expected={"active_watch_both","payload_increased_both","growth_samples",
                  "products_quiesced_both","owner_revocation_verified"}
        if (not isinstance(result,dict) or set(result)!=expected
                or any(result[k] is not True for k in expected-{"growth_samples"})
                or type(result["growth_samples"]) is not int or result["growth_samples"]<3):
            raise RuntimeError("activity_evidence_rejected_preserve_owners")
        self.save({"version":1,"phase":"normal_revoked","checks":result})
        return result

    def stop_owners(self,stop_target,stop_controller):
        state=self.read()
        if state.get("phase") not in ("normal_revoked","owner_stop_outcome_unknown"):
            raise RuntimeError("activity_revocation_not_verified_preserve_signers")
        joined=state.get("joined",{})
        if not isinstance(joined,dict) or any(k not in ("target","controller") or v is not True for k,v in joined.items()):
            raise RuntimeError("activity_join_state_rejected")
        state.update(phase="owner_stop_outcome_unknown",joined=joined)
        self.save(state)
        for role,stop in (("controller",stop_controller),("target",stop_target)):
            if joined.get(role) is True:continue
            # Each callback must prove its actual parent + product + signer
            # zero exit, not merely marker creation or successful SSH transport.
            try:receipt=stop()
            except Exception:raise RuntimeError("activity_owner_join_unconfirmed") from None
            if receipt!={"status":"owner_closed","joined":True,"zero_exit":True} or receipt.get("joined") is not True or receipt.get("zero_exit") is not True:
                raise RuntimeError("activity_owner_join_unconfirmed")
            joined[role]=True;self.save(state)
        state["phase"]="owners_joined_retirement_pending";self.save(state)
        # HTTPS tunnels stay alive for the independent online retirement tool.
        # No identities, node files, tasks or test roots are deleted here.
