import json
import unittest
from unittest.mock import patch

from desktop_stage import DesktopStage, checked_ack


class DesktopStageTests(unittest.TestCase):
    def test_current_challenge_required_and_public_fields_exact(self):
        checked_ack({"nonce":"a"*32,"status":"waiting"},"a"*32)
        for value in ({"nonce":"b"*32,"status":"waiting"}, {"nonce":"a"*32,"status":"ready"}, {"nonce":"a"*32,"status":"waiting","secret":"no"}):
            with self.assertRaises(RuntimeError):checked_ack(value,"a"*32)

    def test_waiter_challenge_precedes_launch_and_no_private_material(self):
        calls=[]
        class Transport:
            def powershell(self,script):
                calls.append(script)
                if 'Get-Content' in script:return json.dumps({"nonce":"a"*32,"status":"waiting"})
                return ''
            def check(self,ok,label):
                if not ok:raise AssertionError(label)
        with patch('desktop_stage.secrets.token_hex',return_value='a'*32):
            stage=DesktopStage(Transport(),r'C:\Users\jarvis\remoteapp-compiled-stage-0123456789abcdef')
        stage.launch('b'*64)
        self.assertIn('staging_root_unusable',calls[0])
        self.assertIn('ping.json',calls[1])
        self.assertIn('ack.json',calls[2])
        self.assertIn('launch.json',calls[3])
        self.assertNotIn('launch.json',calls[1])
        with self.assertRaises(RuntimeError):stage.launch('invalid')
        with self.assertRaises(RuntimeError):stage.write('identity.json',{})

    def test_invalid_root_never_contacts_windows(self):
        class Transport:
            def powershell(self,script):raise AssertionError('unexpected contact')
        for root in ('C:\\',r'C:\Users\jarvis\remoteapp-compiled-stage-0123456789abcdef\..',r'D:\stage'):
            with self.assertRaises(RuntimeError):DesktopStage(Transport(),root)
