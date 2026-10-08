import hashlib
import unittest
from compiled_coordination import BoundHandoff, runtime_inputs

class CompiledCoordinationTests(unittest.TestCase):
    def inputs(self):
        ready=dict(target="a"*64,controller="b"*64,verifier="nodekey:"+"c"*64)
        binding=dict(target_node="nodekey:"+"d"*64,controller_node="nodekey:"+"e"*64,target_ip="100.64.0.2",controller_ip="100.64.0.3")
        grant=dict(claims={name:hashlib.sha256(bytes.fromhex(ready[name])).hexdigest() for name in ("target","controller")},signature="fixture")
        tickets=[dict(credential=dict(Secret="private-target")),dict(credential=dict(Secret="private-controller"))]
        return [ready,binding,grant,r"C:\Users\jarvis\test\nodes\target","/private/tmp/test/nodes/controller",tickets]

    def test_exact_role_and_binding_private_inputs(self):
        args=self.inputs();values=runtime_inputs(*args)
        self.assertTrue(values["target"]["Server"]);self.assertFalse(values["controller"]["Server"])
        self.assertEqual(values["target"]["AuthKey"],"private-target")
        values["controller"]["Binding"]["controller_ip"]="mutated"
        self.assertEqual(args[1]["controller_ip"],"100.64.0.3")
        self.assertNotIn("Certificate",values["target"])
        self.assertNotIn("Endpoint",values["controller"])

    def test_bad_pair_or_paths_rejected(self):
        changes=[lambda a:a[0].update(target="B"*64),lambda a:a[1].update(target_ip="127.0.0.1"),lambda a:a[1].update(target_node=a[1]["controller_node"]),lambda a:a[2]["claims"].update(target="x"),lambda a:a.__setitem__(3,r"C:\test\..\state"),lambda a:a.__setitem__(4,"/private/tmp/../state"),lambda a:a.__setitem__(5,a[5][:1])]
        for change in changes:
            args=self.inputs();change(args)
            with self.assertRaises((RuntimeError,ValueError)):runtime_inputs(*args)

    def test_handoff_waits_both_join_and_never_retries_failure(self):
        events=[];inputs=runtime_inputs(*self.inputs())
        handoff=BoundHandoff(inputs,lambda:events.extend(["controller_joined","target_joined"]))
        handed=handoff.take();self.assertEqual(events,["controller_joined","target_joined"]);self.assertEqual(handed,inputs)
        with self.assertRaises(RuntimeError):handoff.take()
        def failed():raise RuntimeError("join_unconfirmed")
        handoff=BoundHandoff(inputs,failed)
        with self.assertRaises(RuntimeError):handoff.take()
        with self.assertRaises(RuntimeError):handoff.take()

if __name__=="__main__":unittest.main()
