// Dedicated connection identity; never reads existing pairing/GUI credentials.
// Inherited private pipes only. No key export, argv secret or plaintext fallback.
import Foundation
import Security
import CryptoKit
import LocalAuthentication

func output(_ value: [String:String]) throws {
    try FileHandle.standardOutput.write(contentsOf: JSONSerialization.data(withJSONObject:value)+Data([10]))
}
func frame() throws -> [String:Any]? {
    var data=Data()
    while data.count<8192 {
        guard let b=try FileHandle.standardInput.read(upToCount:1),!b.isEmpty else {if data.isEmpty{return nil};throw VaultError.denied}
        if b[0]==10 {return try JSONSerialization.jsonObject(with:data) as? [String:Any]}
        data.append(b)
    }
    throw VaultError.denied
}
enum VaultError:Error {case denied}
func hex(_ data:Data)->String {data.map{String(format:"%02x",$0)}.joined()}
func load(_ query:[String:Any]) throws -> Data? {
    var q=query;q[kSecReturnData as String]=true;q[kSecMatchLimit as String]=kSecMatchLimitOne
    var result:CFTypeRef?
    let status=SecItemCopyMatching(q as CFDictionary,&result)
    if status==errSecItemNotFound{return nil}
    guard status==errSecSuccess,let data=result as? Data,data.count==32 else {throw VaultError.denied}
    return data
}
do {
    guard let cfg=try frame(),Set(cfg.keys)==Set(["Scope","Create","Expected"]),let scope=cfg["Scope"] as? String,scope.count==32,scope.allSatisfy({"0123456789abcdef".contains($0)}),let create=cfg["Create"] as? Bool,let expected=cfg["Expected"] as? String,(create && expected.isEmpty) || (!create && expected.count==64) else {throw VaultError.denied}
    // Fail instead of prompting from a background helper. Interactive release
    // authorization/signing policy remains a separate installer requirement.
    let auth=LAContext();auth.interactionNotAllowed=true
    let q:[String:Any]=[kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"com.personalremotedesktop.connection-identity.poc.v1",kSecAttrAccount as String:scope,kSecUseAuthenticationContext as String:auth]
    var seed=try load(q)
    if seed==nil {
        guard create else {throw VaultError.denied}
        let generated=Curve25519.Signing.PrivateKey().rawRepresentation
        var attrs=q;attrs.removeValue(forKey:kSecUseAuthenticationContext as String);attrs[kSecValueData as String]=generated;attrs[kSecAttrAccessible as String]=kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        let status=SecItemAdd(attrs as CFDictionary,nil)
        guard status==errSecSuccess || status==errSecDuplicateItem else {throw VaultError.denied}
        seed=try load(q) // duplicate creation NEVER overwrites the winning key
    }
    guard let seed else {throw VaultError.denied}
    let key=try Curve25519.Signing.PrivateKey(rawRepresentation:seed)
    let publicHex=hex(key.publicKey.rawRepresentation)
    guard expected.isEmpty || expected==publicHex else {throw VaultError.denied}
    try output(["status":"ready","public_key":publicHex])
    while let c=try frame() {
        guard let op=c["Op"] as? String else {throw VaultError.denied}
        if op=="stop",Set(c.keys)==Set(["Op"]) {try output(["status":"closed"]);exit(0)}
        if op=="delete",create,Set(c.keys)==Set(["Op"]) {
            guard SecItemDelete(q as CFDictionary)==errSecSuccess else {throw VaultError.denied}
            try output(["status":"deleted"]);exit(0)
        }
        guard op=="sign",Set(c.keys)==Set(["Op","Message"]),let value=c["Message"] as? String,let raw=Data(base64Encoded:value),raw.count<=4096,let text=String(data:raw,encoding:.utf8),["remoteapp-invite-request-v1\n", "remoteapp-network-api-v1\nPOST /v1/network\n", "remoteapp-provision-intent-v1\n", "remoteapp-node-possession-v1\n", "remoteapp-session-target-v1\n", "remoteapp-session-controller-v1\n", "remoteapp-session-product-tls-target-v1\n", "remoteapp-session-product-tls-controller-v1\n"].contains(where: {text.hasPrefix($0)}) else {throw VaultError.denied}
        try output(["status":"signed","signature":hex(try key.signature(for:raw))])
    }
} catch {try? output(["status":"failed"]);exit(1)}
