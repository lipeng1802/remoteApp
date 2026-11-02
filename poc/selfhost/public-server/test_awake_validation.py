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
