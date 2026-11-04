import copy
import hashlib
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

from archive_cache import BLOCK,pack,verify


class ByteCacheTransport(unittest.TestCase):
    def fixture(self,root):
        old=root/'old';new=root/'new';old.mkdir();new.mkdir()
        chunk=bytes(range(256))*(BLOCK//256)
        (old/'base.exe').write_bytes(chunk*2)
        (new/'fresh.exe').write_bytes(chunk+b'FRESH INPUT')
        (new/'second.exe').write_bytes(chunk+b'FRESH INPUT')
        (new/'signed-input.json').write_text('{"fresh":true}')
        helper=root/'restore.exe';helper.write_bytes(b'CURRENT RESTORER')
        return old,new,helper

    def test_exact_reconstruction_dedup_and_current_input_pins(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);old,new,helper=self.fixture(root);archive=root/'wire.tar.xz'
            audit=pack(new,archive,old,helper)
            verify(new,archive,audit)
            self.assertEqual(audit['copied_bytes'],2*BLOCK)
            self.assertEqual(audit['literal_bytes'],len(b'FRESH INPUT'))
            with tarfile.open(archive) as bundle:
                plan=json.load(bundle.extractfile('_transport/plan.json'))
                literal=bundle.extractfile('_transport/literal.bin').read()
                self.assertEqual(bundle.extractfile('signed-input.json').read(),(new/'signed-input.json').read_bytes())
                self.assertNotIn('fresh.exe',bundle.getnames())
                for target in plan['targets']:
                    data=bytearray()
                    for source,offset,size in target['steps']:
                        raw=literal if source==0 else (old/plan['sources'][source-1]['name']).read_bytes()
                        data+=raw[offset:offset+size]
                    self.assertEqual(bytes(data),(new/target['name']).read_bytes())
                    self.assertEqual(hashlib.sha256(data).hexdigest(),target['sha256'])
            (new/'fresh.exe').write_bytes(b'CHANGED')
            with self.assertRaisesRegex(RuntimeError,'input_changed'):verify(new,archive,audit)

    def test_corrupt_archive_and_symlink_input_are_not_accepted(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);old,new,helper=self.fixture(root);archive=root/'wire.tar.xz'
            audit=pack(new,archive,old,helper);bad=copy.deepcopy(audit);bad['archive_sha256']='0'*64
            with self.assertRaisesRegex(RuntimeError,'audit_rejected'):verify(new,archive,bad)
            (new/'link.exe').symlink_to(old/'base.exe')
            with self.assertRaisesRegex(RuntimeError,'link_rejected'):pack(new,root/'bad.tar.xz',old,helper)


if __name__=='__main__':unittest.main()
