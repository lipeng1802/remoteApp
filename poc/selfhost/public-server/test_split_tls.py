import unittest
from split_tls import negative_ok,closed_ok

class TLSAssertions(unittest.TestCase):
    def test_negatives_require_specific_rejection_and_no_protected_payload(self):
        before={"pongs":4,"preauth_rejected":0}
        cases=[("pin",{"ended":True,"pongs":0,"pin_rejected":True},dict(before)),
               ("key",{"ended":True,"pongs":0,"result":"authentication_rejected"},dict(before)),
               ("preauth",{"ended":True,"pongs":0},dict(before,preauth_rejected=1))]
        for mode,client,after in cases:
            self.assertTrue(negative_ok(mode,before,after,client))
            self.assertFalse(negative_ok(mode,before,dict(after,pongs=5),client))
            self.assertFalse(negative_ok(mode,before,after,dict(client,pongs=1)))
            self.assertFalse(negative_ok(mode,before,after,dict(client,ended=False)))
        self.assertFalse(negative_ok("key",before,before,{"ended":True,"pongs":0,"result":"timeout"}))
        self.assertFalse(negative_ok("pin",before,before,{"ended":True,"pongs":0}))

    def test_closure_cannot_be_timeout_or_more_pongs(self):
        old={"pongs":20}; final={"ended":True,"result":"connection_closed","pongs":20}
        self.assertTrue(closed_ok(old,final,old,old))
        for changes in ({"result":"timeout"},{"result":None},{"result":"cancelled"},{"result":"pending"},{"ended":False},{"pongs":21}):
            self.assertFalse(closed_ok(old,dict(final,**changes),old,old))
        self.assertFalse(closed_ok(old,final,old,{"pongs":21}))
        self.assertFalse(closed_ok({},dict(final,pongs=None),{},{}))

if __name__=="__main__": unittest.main()
