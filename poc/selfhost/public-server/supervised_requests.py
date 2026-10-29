"""Trusted management adapters. No secret frame/reply is logged or put in argv.

Windows relay is an already verified local binary in the logged-in user's
desktop namespace. SSH transports private JSON; it does not authenticate a
publisher or replace the product's device signatures/OS identity binding.
"""
import json
import re
import subprocess


def unique_object(pairs):
    result = {}
    names = set()
    for key, value in pairs:
        if key.lower() in names:
            raise RuntimeError("supervised_duplicate_reply_field")
        names.add(key.lower())
        result[key] = value
    return result


def reply(raw):
    if not isinstance(raw, bytes) or len(raw) > 8192:
        raise RuntimeError("supervised_reply_bounds")
    try:
        value = json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object)
    except (ValueError, UnicodeError):
        raise RuntimeError("supervised_reply_invalid") from None
    if not isinstance(value, dict):
        raise RuntimeError("supervised_reply_invalid")
    return value


class WindowsRelay:
    def __init__(self, host, executable, pipe):
        # All selectors are trusted public management bootstrap, never frames.
        if (not re.fullmatch(r"[a-zA-Z0-9_.@-]+", host) or host.startswith("-")
                or not re.fullmatch(r"[A-Za-z]:\\[A-Za-z0-9_. \\-]+\.exe", executable)
                or ".." in executable.split("\\")
                or not re.fullmatch(r"\\\\\.\\pipe\\remoteapp-session-[0-9a-f]{32}", pipe)):
            raise RuntimeError("supervised_management_bootstrap_invalid")
        self.command = ["ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host,
                        subprocess.list2cmdline([executable, pipe])]

    def __call__(self, frame):
        raw = json.dumps(frame, separators=(",", ":"), allow_nan=False).encode() + b"\n"
        if len(raw) > 8192:
            raise RuntimeError("supervised_request_bounds")
        try:
            result = subprocess.run(self.command, input=raw, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=35)
        except (OSError, subprocess.TimeoutExpired):
            # Timeout only closes this SSH invocation; never kill the desktop
            # owner or claim its join/authorization outcome is known.
            raise RuntimeError("supervised_management_outcome_unknown") from None
        if result.returncode != 0:
            raise RuntimeError("supervised_management_outcome_unknown")
        return reply(result.stdout)


class LocalSupervisor:
    def __init__(self, joined_pipe):
        self.owner = joined_pipe
        self.pending = False
        self.poisoned = False

    def read(self):
        try:
            result = self.owner.read(timeout=25)
        except RuntimeError as error:
            # A timeout leaves the original frame outstanding. A malformed or
            # terminal response cannot safely be resynchronized as a new call.
            if str(error) != "private_pipe_timeout":
                self.poisoned = True
            raise RuntimeError("supervised_local_outcome_unknown") from None
        except Exception:
            self.poisoned = True
            raise RuntimeError("supervised_local_outcome_unknown") from None
        self.pending = False
        return result

    def __call__(self, frame):
        # Reuse the existing bounded private parser/serialization. Process
        # ownership and actual stop/EOF/exit remain with JoinedPipe's caller.
        with self.owner.lock:
            if self.poisoned:
                raise RuntimeError("supervised_local_pipe_sealed_preserve_owner")
            if self.pending:
                # Consume/discard the old reply BEFORE a new frame is sent.
                # Its contents must never be returned as this call's receipt.
                self.read()
            self.pending = True
            try:
                self.owner.send(frame)
            except Exception:
                self.poisoned = True
                raise RuntimeError("supervised_local_write_unknown_preserve_owner") from None
            return self.read()
