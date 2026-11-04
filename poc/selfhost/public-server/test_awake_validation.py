from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
import run_awake_validation as awake
import run_supervised_activity as r
import fault_activity

class AwakeValidationTests(unittest.TestCase):
    def test_process_scoped_assertion_preserves_worker_exit(self):
        with patch.object(awake.subprocess,'run',return_value=subprocess.CompletedProcess([],7)) as call:
            self.assertEqual(awake.launch(),7)
        args=call.call_args.args[0]
        self.assertEqual(args[:2],['/usr/bin/caffeinate','-i']);self.assertEqual(args[-1],'--worker')
        self.assertNotIn('-d',args);self.assertNotIn('-s',args)
    def test_failed_start_never_replays_or_tears_down_unknown_state(self):
        with patch.object(r,'prepare',return_value=type('Work',(),{'name':'fixture'})()) as prepare,patch.object(r,'start',side_effect=RuntimeError('unknown')) as start,patch.object(r,'exercise') as exercise,patch.object(fault_activity,'stop') as stop,patch.object(fault_activity,'retire') as retire:
            with self.assertRaises(RuntimeError):awake.worker()
            self.assertEqual(prepare.call_count,1);self.assertEqual(start.call_count,1)
            exercise.assert_not_called();stop.assert_not_called();retire.assert_not_called()

    def test_preprovision_terminal_runs_zero_exit_cleanup_without_pinned_revoke(self):
        from activity_control import Activity
        work=Path('fixture')
        with patch.object(r,'prepare',return_value=work),patch.object(r,'start'),patch.object(r,'exercise',side_effect=RuntimeError('preprovision')),patch.object(Activity,'read',return_value={'phase':'preprovision_terminal'}),patch.object(r,'stop') as stop,patch.object(r,'retire') as retire,patch.object(fault_activity,'revoke') as revoke,patch.object(fault_activity,'stop') as fault_stop:
            awake.worker()
            stop.assert_called_once_with(work)
            retire.assert_called_once_with(work)
            revoke.assert_not_called()
            fault_stop.assert_not_called()

    def test_uncertain_removal_never_becomes_fault_stop_or_intact_retirement(self):
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            work=Path(directory);(work/'program-removal-intent.json').write_text('{}')
            with patch.object(r,'exercise',side_effect=RuntimeError('unknown')),patch.object(r,'stop') as stop,patch.object(r,'retire') as retire,patch.object(fault_activity,'revoke') as revoke,patch.object(fault_activity,'stop') as fault_stop:
                with self.assertRaisesRegex(RuntimeError,'program_removal_unknown_preserve_original_owners'):awake.finish(work)
                stop.assert_not_called();retire.assert_not_called();revoke.assert_not_called();fault_stop.assert_not_called()
