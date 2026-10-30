import base64
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import tarfile
import time
import unittest
from unittest.mock import patch

from activity_control import Activity
import run_supervised_activity as runner


class ManagementActivityGates(unittest.TestCase):
    def test_powershell_declares_utf8_before_public_localized_diagnostics(self):
        with patch.object(runner,'command',return_value='{"status":"已恢复"}'.encode('utf8')) as call:
            self.assertEqual(runner.ps("'PUBLIC'"),'{"status":"已恢复"}')
            encoded=call.call_args.args[0][-1].split()[-1]
            script=base64.b64decode(encoded).decode('utf-16le')
            self.assertTrue(script.startswith('$OutputEncoding=[Text.UTF8Encoding]::new($false);[Console]::OutputEncoding=$OutputEncoding;'))
            self.assertTrue(script.endswith("'PUBLIC'"))

    def test_reuse_requires_verified_exact_immutable_inputs_and_omits_only_those(self):
        with tempfile.TemporaryDirectory() as d:
            work=Path(d);source=work/'windows';(source/'payload').mkdir(parents=True)
            (source/'payload/identity-vault.exe').write_bytes(b'fixed helper')
            (source/'payload/supervisor.exe').write_bytes(b'current supervisor')
            (source/'fixture.json').write_bytes(b'fresh profile')
            for result in ('INPUTS_VERIFIED','WRONG',RuntimeError('unavailable')):
                with self.subTest(result=str(result)),patch.object(runner,'ps',side_effect=result if isinstance(result,Exception) else None,return_value=result) as call:
                    if result!='INPUTS_VERIFIED':
                        with self.assertRaisesRegex(RuntimeError,'input_verification_unavailable'):runner.verified_windows_inputs(work)
                        continue
                    old,items=runner.verified_windows_inputs(work)
                    self.assertNotIn('installed',old)
                    self.assertEqual(len(items),1 if result=='INPUTS_VERIFIED' else 0)
                    self.assertIn('Get-FileHash',call.call_args.args[0])
                    if items:
                        self.assertEqual(items[0]['relative'],'payload/identity-vault.exe')
                        runner.pack_windows(source,work/'reused.tar.xz',omit={x['relative'] for x in items})
                        with tarfile.open(work/'reused.tar.xz') as archive:
                            self.assertEqual(set(archive.getnames()),{'fixture.json','payload/supervisor.exe'})

    def test_test_compiler_wrapper_preserves_embedded_flags_and_other_commands(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);fake=root/'fake-go'
            fake.write_text('#!'+sys.executable+'\nimport json,sys\nprint(json.dumps(sys.argv[1:]))\n');fake.chmod(0o500)
            wrapper=runner.test_build_wrapper(root,fake)
            for args,expected in ((['version'],['version']),(['build','./cmd/tool'],['build','-ldflags=-s -w','./cmd/tool']),(['build','-ldflags=-X main.embeddedRelease=PUBLIC -X main.embeddedProductDigest=HASH','./cmd/tool'],['build','-ldflags=-s -w -X main.embeddedRelease=PUBLIC -X main.embeddedProductDigest=HASH','./cmd/tool'])):
                self.assertEqual(json.loads(subprocess.check_output([str(wrapper),*args])),expected)
            with self.assertRaises(FileExistsError):runner.test_build_wrapper(root,fake)

    def test_solid_archive_roundtrips_exact_regular_files_and_refuses_symlink(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);source=root/"source";source.mkdir();(source/"payload").mkdir()
            (source/"payload/tool.exe").write_bytes(bytes(range(256))*1000)
            (source/"fixture.json").write_text('{"test":true}')
            runner.pack_windows(source,root/"bundle.tar.xz")
            with tarfile.open(root/"bundle.tar.xz") as archive:
                self.assertEqual(set(archive.getnames()),{"payload/tool.exe","fixture.json"})
                for member in archive:
                    self.assertTrue(member.isfile())
                    self.assertEqual(archive.extractfile(member).read(),(source/member.name).read_bytes())
            (source/"foreign").symlink_to(root/"outside")
            with self.assertRaisesRegex(RuntimeError,"archive_link_rejected"):runner.pack_windows(source,root/"bad.tar.xz")

    def fixture(self,directory,phase="built",expires=None):
        w=Path(directory);n="a"*12
        m=dict(nonce=n,win_dir="C:\\Users\\jarvis\\remoteapp-r75-package-"+n,
               win_root="C:\\Users\\jarvis\\remoteapp-r75-"+n,
               mac_root="/private/tmp/remoteapp-r75-"+n+"/installed",
               holder_dir="/private/tmp/remoteapp-r75-"+n+"/holder")
        (w/"management.json").write_text(json.dumps(m));Activity(w).save({"version":1,"phase":phase})
        for target in ("darwin","windows"):
            (w/target).mkdir()
            spec={"test_namespace":True,"bundled_profile":{"claims":{"expires_at":expires or int(time.time())+1200}}}
            (w/target/"fixture.json").write_text(json.dumps({"release":base64.b64encode(json.dumps(spec).encode()).decode()}))
        return w,m

    def test_fixed_roots_reject_overrides_before_remote_access(self):
        for field in ("win_root","win_dir","mac_root","holder_dir"):
            with self.subTest(field=field),tempfile.TemporaryDirectory() as d:
                w,m=self.fixture(d);m[field]="unknown-existing-root";(w/"management.json").write_text(json.dumps(m))
                with patch.object(runner,"command") as call:
                    with self.assertRaisesRegex(RuntimeError,"fixed_roots_invalid"):runner.start(w)
                    call.assert_not_called()

    def test_short_build_and_startup_unknown_cannot_restart_owners(self):
        for phase,expires in (("built",int(time.time())+30),("startup_outcome_unknown",int(time.time())+1200)):
            with self.subTest(phase=phase),tempfile.TemporaryDirectory() as d:
                w,_=self.fixture(d,phase,expires)
                with patch.object(runner,"command") as call:
                    with self.assertRaisesRegex(RuntimeError,"fresh_build_required"):runner.start(w)
                    call.assert_not_called()

    def test_unknown_exercise_cannot_stop_or_delete_anything(self):
        with tempfile.TemporaryDirectory() as d:
            w,_=self.fixture(d,"exercise_outcome_unknown")
            with patch.object(runner,"ps") as ps,patch.object(runner,"command") as command:
                with self.assertRaisesRegex(RuntimeError,"preserve_signers"):runner.stop(w)
                with self.assertRaisesRegex(RuntimeError,"join_before_retirement"):runner.retire(w)
                ps.assert_not_called();command.assert_not_called()

    def test_uncertain_destructive_retirement_cannot_be_implicitly_repeated(self):
        with tempfile.TemporaryDirectory() as d:
            w,_=self.fixture(d,"retirement_outcome_unknown")
            with patch.object(runner,"command") as command:
                with self.assertRaisesRegex(RuntimeError,"join_before_retirement"):runner.retire(w)
                command.assert_not_called()

    def test_large_powershell_requires_file_upload(self):
        with patch.object(runner,"command") as command:
            with self.assertRaisesRegex(RuntimeError,"requires_file_upload"):runner.ps("x"*3000)
            command.assert_not_called()

    def test_transport_failures_hide_private_outputs(self):
        for failure in (subprocess.TimeoutExpired([],90),OSError("PRIVATE")):
            with patch.object(runner.subprocess,"run",side_effect=failure):
                with self.assertRaisesRegex(RuntimeError,"^activity_management_outcome_unknown$"):runner.command(["trusted"])
        with patch.object(runner.subprocess,"run",return_value=subprocess.CompletedProcess([],1,b"PRIVATE",b"PRIVATE")):
            with self.assertRaisesRegex(RuntimeError,"^activity_management_command_failed$"):runner.command(["trusted"])

    def test_mac_setup_saves_only_strict_fixed_outcome_and_never_replays(self):
        cases=[(1,b'{"status":"setup_failed_state_preserved","stage":"native_enrollment"}',"native_enrollment"),
               (1,b'{"status":"setup_failed_state_preserved","stage":"PRIVATE"}',"unavailable"),
               (1,b'{"status":"setup_failed_state_preserved","stage":"native_trust","PRIVATE":"PRIVATE"}',"unavailable"),
               (0,b'{"status":"test_installed"}',"completed"),
               (1,b'{"status":"test_installed"}',"unavailable"),(1,b'PRIVATE',"unavailable")]
        for code,raw,want in cases:
            with self.subTest(want=want),tempfile.TemporaryDirectory() as d:
                w=Path(d);m={"mac_root":str(w/"installed")}
                with patch.object(runner.subprocess,'run',return_value=subprocess.CompletedProcess([],code,raw,b'PRIVATE')) as call:
                    if want=='completed':runner.setup_mac(w,m)
                    else:
                        with self.assertRaisesRegex(RuntimeError,'setup_unconfirmed'):runner.setup_mac(w,m)
                    text=(w/'mac-setup-result.json').read_text()
                    self.assertNotIn('PRIVATE',text);self.assertEqual(json.loads(text)['stage'],want)
                    with self.assertRaisesRegex(RuntimeError,'explicit_recovery_required'):runner.setup_mac(w,m)
                    self.assertEqual(call.call_count,1)

    def test_mac_setup_unknown_attempt_cannot_restart_and_existing_root_not_called(self):
        for partial in (False,True):
            with tempfile.TemporaryDirectory() as d:
                w=Path(d);m={"mac_root":str(w/'installed')}
                if partial:Path(m['mac_root']).mkdir()
                with patch.object(runner.subprocess,'run',side_effect=OSError('PRIVATE')) as call:
                    if partial:
                        with self.assertRaisesRegex(RuntimeError,'explicit_recovery_required'):runner.setup_mac(w,m)
                        call.assert_not_called()
                    else:
                        with self.assertRaisesRegex(RuntimeError,'setup_outcome_unknown'):runner.setup_mac(w,m)
                        with self.assertRaises(FileExistsError):runner.setup_mac(w,m)
                        self.assertEqual(call.call_count,1)
                        self.assertFalse((w/'mac-setup-result.json').exists())

    def test_current_helpers_build_before_profiles_and_keep_fixed_platform_wiring(self):
        with tempfile.TemporaryDirectory() as d:
            w=Path(d);calls=[]
            def command(args,timeout=90):
                calls.append(args)
                if args[0]=='env':Path(args[args.index('-o')+1]).write_bytes(('CURRENT '+args[-1]+args[1]).encode())
                return b''
            with patch.object(runner,'command',side_effect=command):
                audit=runner.build_current_helpers(w,Path('/trusted/go'))
            self.assertEqual(set(audit['helpers']),{'darwin','windows'})
            self.assertEqual(len(calls),6)
            self.assertTrue(all('CGO_ENABLED=1' in a for a in calls if a[0]=='env' and a[1]=='GOOS=darwin'))
            self.assertTrue(all('CGO_ENABLED=0' in a for a in calls if a[0]=='env' and a[1]=='GOOS=windows'))
            self.assertFalse((w/'service-profile.json').exists())
            with self.assertRaises(FileExistsError):runner.build_current_helpers(w,Path('/trusted/go'))

    def test_new_helpers_are_uploaded_and_never_silently_reused_from_old_cache(self):
        import hashlib
        with tempfile.TemporaryDirectory() as d:
            w=Path(d);payload=w/'windows/payload';payload.mkdir(parents=True)
            for name in ('trust-journal.exe','identity-vault.exe','tls-product.exe'):(payload/name).write_bytes(name.encode())
            helpers={}
            for platform in ('darwin','windows'):
                helpers[platform]={n+('.exe' if platform=='windows' else ''):hashlib.sha256((n+'.exe').encode()).hexdigest() for n in ('trust-journal','identity-vault')}
            audit={'version':1,'helpers':helpers};(w/'current-helpers.json').write_text(json.dumps(audit))
            with patch.object(runner,'ps',return_value='INPUTS_VERIFIED') as call:
                _,items=runner.verified_windows_inputs(w)
                self.assertEqual([x['name'] for x in items],['tls-product.exe'])
                self.assertNotIn('trust-journal.exe',call.call_args.args[0])
                (payload/'identity-vault.exe').write_bytes(b'CHANGED')
                with self.assertRaisesRegex(RuntimeError,'current_helper_changed'):runner.verified_windows_inputs(w)
                self.assertEqual(call.call_count,1)
                helpers['windows']['tls-product.exe']='a'*64
                (w/'current-helpers.json').write_text(json.dumps(audit))
                with self.assertRaisesRegex(RuntimeError,'helper_audit_rejected'):runner.verified_windows_inputs(w)
                self.assertEqual(call.call_count,1)


if __name__=="__main__":unittest.main()
