"""One explicitly authorized TEST round with process-scoped idle-sleep inhibition.

No global power settings, display/lock prevention, deadline changes, or finally
teardown. Manual sleep/lid closure can still invalidate the signed short window.
"""
from pathlib import Path
import subprocess
import sys


def launch():
    # caffeinate releases its idle-system assertion when this worker exits.
    # Native owners retain normal priority; build children apply their limits.
    return subprocess.run(['/usr/bin/caffeinate','-i',sys.executable,str(Path(__file__).resolve()),'--worker']).returncode


def worker():
    import run_supervised_activity as r
    import fault_activity as fault
    work=r.prepare()
    print('FRESH_WORK '+work.name,flush=True)
    r.start(work);print('OWNERS_READY',flush=True)
    try:r.exercise(work)
    except Exception:
        print('EXERCISE_FAILED',flush=True)
        fault.revoke(work);print('FAULT_REVOKED',flush=True)
        fault.stop(work);print('FAULT_JOINED',flush=True)
        fault.retire(work);print('FAULT_RETIRED',flush=True)
    else:
        print('NORMAL_REVOKED',flush=True)
        r.stop(work);print('NORMAL_JOINED',flush=True)
        r.retire(work);print('NORMAL_RETIRED',flush=True)


if __name__=='__main__':
    if sys.argv[1:]==['--worker']:
        try:worker()
        except Exception:raise SystemExit('awake_round_failed_consult_saved_state') from None
    elif not sys.argv[1:]:raise SystemExit(launch())
    else:raise SystemExit('explicit_awake_round_required')
