import unittest
from native_bootstrap import NativeIdentities, checked_windows_bootstrap, checked_desktop_bootstrap


class NativeBootstrapTests(unittest.TestCase):
    def test_identity_and_compiled_session_scopes_are_distinct(self):
        folder=r"C:\test";identity=dict(ExpectedPublic="b"*64,VaultScope="a"*32)
        value=dict(status="ready",scope="c"*32,identity_scope="a"*32,public_key="b"*64,pipe="\\\\.\\pipe\\remoteapp-session-"+"c"*32,channel="desktop-test-"+"d"*32,epoch=1,relay_executable=folder+"\\desktop-relay.exe")
        self.assertEqual(checked_desktop_bootstrap(value,identity,folder)["Scope"],"c"*32)
        for change in ({"identity_scope":"e"*32},{"public_key":"f"*64},{"epoch":True},{"ProductKey":"secret"},{"root":"peer"}):
            with self.assertRaises(RuntimeError):checked_desktop_bootstrap(dict(value,**change),identity,folder)
        # The already validated 4T r2 binary used one scope; it is still usable
        # as the identity creator during new compiled bundle preparation.
        legacy=dict(value,scope="a"*32,pipe="\\\\.\\pipe\\remoteapp-session-"+"a"*32);del legacy["identity_scope"]
        self.assertEqual(checked_desktop_bootstrap(legacy,identity,folder)["Expected"],"b"*64)

    def test_fixed_public_bootstrap(self):
        folder = r"H:\tests"
        value = dict(status="ready", scope="ab" * 16, public_key="cd" * 32,
                     pipe="\\\\.\\pipe\\remoteapp-vault-" + "ab" * 16,
                     relay_executable=folder + "\\vault-relay.exe")
        cfg = checked_windows_bootstrap(value, folder)
        self.assertNotIn("AppKey", cfg)
        for field, bad in (("scope", "x"), ("public_key", "CD"*32), ("pipe", "remote"),
                           ("relay_executable", "C:\\other.exe"), ("status", "failed")):
            changed = dict(value); changed[field] = bad
            with self.assertRaises(RuntimeError): checked_windows_bootstrap(changed, folder)

    def test_platform_role_isolation(self):
        identities = NativeIdentities.__new__(NativeIdentities)
        identities.target = {"ExpectedPublic": "windows"}
        identities.controller = {"ExpectedPublic": "mac"}
        identities.wrong = {"ExpectedPublic": "wrong-mac"}
        self.assertEqual(identities.config("native-target", True), identities.target)
        self.assertEqual(identities.config("native-controller", False), identities.controller)
        self.assertEqual(identities.config("native-target", False), identities.wrong)
        for role, windows in (("native-controller",True),("private-key",False)):
            with self.assertRaises(RuntimeError): identities.config(role,windows)


if __name__ == "__main__": unittest.main()
