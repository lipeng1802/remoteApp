import unittest
from native_bootstrap import NativeIdentities, checked_windows_bootstrap


class NativeBootstrapTests(unittest.TestCase):
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
