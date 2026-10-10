import contextlib
import io
import unittest

from coordination_diagnostics import run_stage


class CoordinationDiagnosticsTests(unittest.TestCase):
    def test_fixed_markers_do_not_log_private_result_or_error(self):
        output=io.StringIO()
        error=RuntimeError("private-key-and-config")
        def fail(): raise error
        with contextlib.redirect_stdout(output):
            self.assertEqual(run_stage("bound_controller_join",lambda:"private-result"),"private-result")
            with self.assertRaises(RuntimeError) as caught:
                run_stage("bound_target_join",fail)
        self.assertIs(caught.exception,error)
        self.assertNotIn("private",output.getvalue())
        self.assertIn("coordination_bound_controller_join_completed",output.getvalue())
        self.assertIn("coordination_bound_target_join_failed",output.getvalue())
        self.assertNotIn("coordination_bound_target_join_completed",output.getvalue())

    def test_unknown_marker_cannot_execute_or_leak(self):
        output=io.StringIO()
        with contextlib.redirect_stdout(output),self.assertRaises(RuntimeError):
            run_stage("private injected stage",lambda:self.fail("executed"))
        self.assertEqual(output.getvalue(),"")
