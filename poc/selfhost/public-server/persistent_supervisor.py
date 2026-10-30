"""Private macOS/Unix TEST management owner, independent of one SSH/runner call.

An unavailable client/unknown business reply never releases the native signer.
Only an explicit owner_stop attempts normal join. This is same-user management,
not a publisher-authenticated product endpoint or a production login service.
"""
import json
import os
import re
from pathlib import Path
import socket
import stat
import sys

from joined_pipe import JoinedPipe
from supervised_requests import LocalSupervisor, NativeUnix, reply


def private_directory(path):
    path = Path(path)
    if not path.is_absolute() or str(path.resolve(strict=True)) != str(path):
        raise RuntimeError("holder_directory_rejected")
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
        raise RuntimeError("holder_directory_rejected")
    return path


def frame(connection):
    raw = bytearray()
    while b"\n" not in raw:
        chunk = connection.recv(4096)
        if not chunk:
            raise RuntimeError("holder_request_incomplete")
        raw.extend(chunk)
        if len(raw) > 8192:
            raise RuntimeError("holder_request_bounds")
    line, rest = raw.split(b"\n", 1)
    if rest:
        raise RuntimeError("holder_request_extra_frame")
    return reply(bytes(line))


class Owner:
    def __init__(self, child, endpoint=None):
        self.child = child
        self.endpoint = endpoint if endpoint is not None else LocalSupervisor(child)
        self.joined = False
        self.fault = False

    def call(self, request):
        if request == {"Op": "owner_status"}:
            return {"status": "owner_status", "joined": self.joined}
        if request == {"Op": "owner_stop"}:
            if getattr(self.child,'fault_stop_sent',False) or self.fault:
                raise RuntimeError('holder_fault_stop_not_normal_close')
            if not self.joined:
                # An outstanding/malformed business frame cannot become a stop
                # acknowledgement. Preserve ownership for explicit recovery.
                if isinstance(self.endpoint, LocalSupervisor) and self.endpoint.poisoned:
                    raise RuntimeError("holder_pipe_sealed_preserve_owner")
                if isinstance(self.endpoint, LocalSupervisor) and self.endpoint.pending:
                    with self.child.lock:
                        self.endpoint.read()
                self.child.stop()
                self.child.proc.stdout.close()
                self.joined = True
            return {"status": "owner_closed", "joined": True, "zero_exit": True}
        if request == {"Op":"owner_stop_fault"}:
            if self.joined and not self.fault:raise RuntimeError('holder_normal_close_not_fault')
            if not self.joined:
                if isinstance(self.endpoint,LocalSupervisor) and (self.endpoint.poisoned or self.endpoint.pending):
                    raise RuntimeError('holder_pipe_sealed_preserve_owner')
                self.child.stop_fault()
                self.child.proc.stdout.close()
                self.joined=True;self.fault=True
            return {'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1}
        if self.joined or not isinstance(request, dict) or request.get("Op") not in {
                "device", "network", "provision", "product_count", "revoke_session", "quiesce_session"}:
            raise RuntimeError("holder_request_rejected")
        return self.endpoint(request)


def serve(directory, command, child_factory=JoinedPipe, *, require_native=True):
    directory = private_directory(directory)
    address, ready_file = directory/"owner.sock", directory/"owner.json"
    fault_file=directory/'owner-fault-joined.json'
    # Preflight before starting any process/native ownership. Never replace an
    # old socket/bootstrap or silently adopt another retained owner's directory.
    if any(p.exists() or p.is_symlink() for p in (address,ready_file,fault_file)):
        raise RuntimeError("holder_metadata_exists")
    listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    listener.bind(str(address))
    os.chmod(address, 0o600)
    listener.listen(4)
    socket_identity = address.lstat()
    child, owner = None, None
    published = None
    try:
        child = child_factory(command, {"Op": "open"})
        ready = child.read()
        if (not isinstance(ready, dict) or ready.get("status") != "ready"
                or not isinstance(ready.get("vault_scope"), str) or not re.fullmatch(r"[0-9a-f]{32}",ready["vault_scope"])
                or not isinstance(ready.get("public_key"), str) or not re.fullmatch(r"[0-9a-f]{64}",ready["public_key"])):
            raise RuntimeError("holder_owner_not_ready_preserve_state")
        native = NativeUnix(ready["control_socket"]) if require_native else None
        owner = Owner(child, native)
        # Publish public discovery only; authority material and application
        # keys never enter this file. Exclusive file remains after unjoined exit.
        metadata = {"version": 1, "process_id": os.getpid(),
                    "vault_scope": ready["vault_scope"], "public_key": ready["public_key"]}
        if require_native:
            metadata.update(control_socket=ready["control_socket"], supervisor_process_id=child.proc.pid,fault_protocol=1)
        published = json.dumps(metadata, separators=(",", ":")).encode()
        pending_file=directory/("owner.json."+str(os.getpid())+".new")
        with pending_file.open("xb") as output:
            os.chmod(pending_file, 0o600)
            output.write(published)
            output.flush()
            os.fsync(output.fileno())
        # Atomic, no-overwrite publication within our same private directory.
        os.link(pending_file,ready_file)
        pending_file.unlink()
        while not owner.joined:
            connection, unused = listener.accept()
            with connection:
                connection.settimeout(35)
                try:
                    result = owner.call(frame(connection))
                except Exception:
                    result = {"status": "holder_outcome_unknown_state_retained"}
                try:
                    connection.sendall(json.dumps(result, separators=(",", ":")).encode()+b"\n")
                except OSError:
                    # A lost receipt is not an instruction to stop the owner.
                    pass
        if owner.fault:
            receipt={'version':1,'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1,
                     'supervisor_process_id':child.proc.pid,'holder_process_id':os.getpid()}
            pending_fault=directory/('owner-fault-joined.json.'+str(os.getpid())+'.new')
            with pending_fault.open('xb') as output:
                os.chmod(pending_fault,0o600)
                output.write(json.dumps(receipt,separators=(',',':')).encode())
                output.flush();os.fsync(output.fileno())
            os.link(pending_fault,fault_file);pending_fault.unlink()
            descriptor=os.open(directory,os.O_RDONLY)
            try:os.fsync(descriptor)
            finally:os.close(descriptor)
        # Normal or dedicated fault protocol proved actual exit before cleanup.
        if ready_file.read_bytes() != published:
            raise RuntimeError("holder_metadata_changed")
        ready_file.unlink()
        return owner.fault
    finally:
        listener.close()
        # This function is intended to remain resident, not exit on a business
        # failure. Unexpected parent failure is NOT a successful owner join.
        # Do not issue stop/delete in a generic finally or unlink an unknown root.
        if child is None or owner is not None and owner.joined:
            info = address.lstat()
            if (info.st_dev, info.st_ino) == (socket_identity.st_dev, socket_identity.st_ino):
                address.unlink()


class Endpoint:
    def __init__(self, directory):
        self.directory = private_directory(directory)

    def __call__(self, request):
        address = self.directory/"owner.sock"
        info = address.lstat()
        if not stat.S_ISSOCK(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600:
            raise RuntimeError("holder_socket_rejected")
        raw = json.dumps(request, separators=(",", ":"), allow_nan=False).encode()+b"\n"
        if len(raw) > 8192:
            raise RuntimeError("holder_request_bounds")
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
            connection.settimeout(40)
            connection.connect(str(address))
            connection.sendall(raw)
            return frame(connection)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("trusted private directory and verified supervisor path required")
    try:
        if serve(sys.argv[1], [sys.argv[2]]):raise SystemExit(1)
    except Exception:
        raise SystemExit("holder_failed_join_unconfirmed_state_preserved") from None
