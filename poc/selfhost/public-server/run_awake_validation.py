"""One explicitly authorized TEST round with process-scoped idle-sleep inhibition.

No global power settings, display/lock prevention, deadline changes, or finally
teardown. Manual sleep/lid closure can still invalidate the signed short window.
"""
from pathlib import Path
import subprocess
import sys


def launch(preprovision_fault=False,program_removal=False):
    # caffeinate releases its idle-system assertion when this worker exits.
    # Native owners retain normal priority; build children apply their limits.
    args=['/usr/bin/caffeinate','-i',sys.executable,str(Path(__file__).resolve()),'--worker']
    if preprovision_fault:args.append('--preprovision-check')
    if program_removal:args.append('--program-removal')
    return subprocess.run(args).returncode


def worker(preprovision_fault=False,program_removal=False):
    import run_supervised_activity as r
    import fault_activity as fault
    work=r.prepare(program_removal=True) if program_removal else r.prepare()
    print('FRESH_WORK '+work.name,flush=True)
    r.start(work);print('OWNERS_READY',flush=True)
    finish(work,preprovision_fault)


def finish(work,preprovision_fault=False):
    import run_supervised_activity as r
    import fault_activity as fault
    try:
        if preprovision_fault:r.exercise(work,preprovision_fault=True)
        else:r.exercise(work)
    except Exception:
        print('EXERCISE_FAILED',flush=True)
        if (work/'program-removal-intent.json').exists():
            print('REMOVAL_OUTCOME_UNKNOWN_OWNERS_RETAINED',flush=True)
            raise RuntimeError('program_removal_unknown_preserve_original_owners') from None
        from activity_control import Activity
        if Activity(work).read()['phase']=='preprovision_terminal':
            r.stop(work);print('PREPROVISION_JOINED',flush=True)
            r.retire(work);print('PREPROVISION_RETIRED',flush=True)
            return
        fault.revoke(work);print('FAULT_REVOKED',flush=True)
        fault.stop(work);print('FAULT_JOINED',flush=True)
        fault.retire(work);print('FAULT_RETIRED',flush=True)
    else:
        print('NORMAL_REVOKED',flush=True)
        r.stop(work);print('NORMAL_JOINED',flush=True)
        r.retire(work);print('NORMAL_RETIRED',flush=True)


if __name__=='__main__':
    if sys.argv[1:] in (['--worker'],['--worker','--preprovision-check'],['--worker','--program-removal']):
        try:worker(preprovision_fault='--preprovision-check' in sys.argv[1:],program_removal='--program-removal' in sys.argv[1:])
        except Exception:raise SystemExit('awake_round_failed_consult_saved_state') from None
    elif sys.argv[1:]==['--preprovision-check']:raise SystemExit(launch(True))
    elif sys.argv[1:]==['--program-removal']:raise SystemExit(launch(program_removal=True))
    elif not sys.argv[1:]:raise SystemExit(launch())
    else:raise SystemExit('explicit_awake_round_required')
