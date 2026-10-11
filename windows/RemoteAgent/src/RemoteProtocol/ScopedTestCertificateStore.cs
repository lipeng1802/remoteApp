using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

namespace RemoteProtocol;

// Test-only, exact random compiled release/grant namespace. Never reads the
// production Agent certificate. Private keys stay in the current user's OS
// key container; no PFX/file/parent export interface is exposed.
public static class ScopedTestCertificateStore
{
    private static string Subject(string scope)
    {
        if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException();
        if (scope.Length != 64 || scope.Any(c => c is not (>= '0' and <= '9') and not (>= 'a' and <= 'f')))
            throw new InvalidDataException("Invalid test certificate scope.");
        return "CN=Personal Remote Desktop Test " + scope;
    }

    public static X509Certificate2 Open(string scope, bool create)
    {
        var subject = Subject(scope);
        using var mutex = new Mutex(false, "Global\\RemoteAppTestCertificate-" + scope);
        try { if (!mutex.WaitOne(TimeSpan.FromSeconds(3))) throw new InvalidDataException("Test certificate busy."); } catch (AbandonedMutexException) { }
        try
        {
            using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
            store.Open(OpenFlags.ReadWrite);
            var matches = store.Certificates.Find(X509FindType.FindBySubjectDistinguishedName, subject, false);
            if (create)
            {
                if (matches.Count != 0)
                {
                    foreach (var match in matches) match.Dispose();
                    throw new InvalidDataException("Test certificate already exists.");
                }
                var certificate = AgentCertificateFactory.CreateScopedPersistentSelfSigned(subject);
                try { store.Add(certificate); return certificate; }
                catch { try { DeleteKey(certificate); } finally { certificate.Dispose(); } throw; }
            }
            if (matches.Count != 1)
            {
                foreach (var match in matches) match.Dispose();
                throw new InvalidDataException("Test certificate missing or ambiguous.");
            }
            var existing = matches[0];
            if (!existing.HasPrivateKey || existing.NotBefore.ToUniversalTime() > DateTime.UtcNow ||
                existing.NotAfter.ToUniversalTime() <= DateTime.UtcNow)
            {
                existing.Dispose();
                throw new InvalidDataException("Test certificate unavailable.");
            }
            return existing;
        }
        finally { mutex.ReleaseMutex(); }
    }

    // Only after the owning product has actually joined; never grant/pin repair.
    public static void Delete(string scope)
    {
        var subject = Subject(scope);
        using var mutex = new Mutex(false, "Global\\RemoteAppTestCertificate-" + scope);
        try { if (!mutex.WaitOne(TimeSpan.FromSeconds(3))) throw new InvalidDataException("Test certificate busy."); } catch (AbandonedMutexException) { }
        try
        {
            using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
            store.Open(OpenFlags.ReadWrite);
            var matches = store.Certificates.Find(X509FindType.FindBySubjectDistinguishedName, subject, false);
            if (matches.Count > 1)
            {
                foreach (var match in matches) match.Dispose();
                throw new InvalidDataException("Ambiguous test certificate.");
            }
            foreach (var certificate in matches)
            {
                using (certificate)
                {
                    DeleteKey(certificate);
                    store.Remove(certificate);
                }
            }
        }
        finally { mutex.ReleaseMutex(); }
    }

    private static void DeleteKey(X509Certificate2 certificate)
    {
        if (!certificate.HasPrivateKey) return;
        using var rsa = certificate.GetRSAPrivateKey();
        if (rsa is RSACng cng) cng.Key.Delete();
        else if (rsa is RSACryptoServiceProvider csp) { csp.PersistKeyInCsp = false; csp.Clear(); }
        else throw new InvalidDataException("Unsupported test key container.");
    }

    public static void SelfTest()
    {
        var scope = Convert.ToHexString(RandomNumberGenerator.GetBytes(32)).ToLowerInvariant();
        var other = Convert.ToHexString(RandomNumberGenerator.GetBytes(32)).ToLowerInvariant();
        var created = false;
        try
        {
            try { using var missing = Open(scope, false); throw new Exception("Missing scope reopened."); }
            catch (InvalidDataException) { }
            byte[] certificate;
            using (var first = Open(scope, true)) { created = true; certificate = first.RawData; }
            using (var reopened = Open(scope, false))
            {
                if (!reopened.RawData.AsSpan().SequenceEqual(certificate)) throw new Exception("Certificate changed.");
                using var key = reopened.GetRSAPrivateKey() ?? throw new Exception("Missing key.");
                using var publicKey = reopened.GetRSAPublicKey()!;
                var message = RandomNumberGenerator.GetBytes(32);
                if (!publicKey.VerifyData(message, key.SignData(message, HashAlgorithmName.SHA256, RSASignaturePadding.Pss), HashAlgorithmName.SHA256, RSASignaturePadding.Pss))
                    throw new Exception("Native key proof failed.");
            }
            try { using var duplicate = Open(scope, true); throw new Exception("Duplicate initialization accepted."); }
            catch (InvalidDataException) { }
            try { using var foreign = Open(other, false); throw new Exception("Foreign scope accepted."); }
            catch (InvalidDataException) { }
            Delete(scope); created = false;
            try { using var removed = Open(scope, false); throw new Exception("Removed scope reopened."); }
            catch (InvalidDataException) { }
        }
        finally { if (created) Delete(scope); }
    }
}
