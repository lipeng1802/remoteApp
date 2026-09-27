using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

namespace RemoteProtocol;

public static class AgentCertificateFactory
{
    public const string SubjectName = "CN=Personal Remote Desktop Agent";
    private const string ServerAuthenticationOid = "1.3.6.1.5.5.7.3.1";

    public static X509Certificate2 CreateSelfSigned(
        DateTimeOffset? notBefore = null,
        DateTimeOffset? notAfter = null)
    {
        using var rsa = RSA.Create(2048);
        var request = new CertificateRequest(
            SubjectName,
            rsa,
            HashAlgorithmName.SHA256,
            RSASignaturePadding.Pkcs1);
        request.CertificateExtensions.Add(new X509BasicConstraintsExtension(false, false, 0, true));
        request.CertificateExtensions.Add(new X509KeyUsageExtension(
            X509KeyUsageFlags.DigitalSignature | X509KeyUsageFlags.KeyEncipherment,
            true));
        request.CertificateExtensions.Add(new X509EnhancedKeyUsageExtension(
            new OidCollection { new Oid(ServerAuthenticationOid) },
            true));
        request.CertificateExtensions.Add(new X509SubjectKeyIdentifierExtension(request.PublicKey, false));

        var start = notBefore ?? DateTimeOffset.UtcNow.AddMinutes(-5);
        var end = notAfter ?? start.AddYears(5);
        return request.CreateSelfSigned(start, end);
    }
}

public static class AgentCertificateStore
{
    public static X509Certificate2 LoadOrCreate()
    {
        using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
        store.Open(OpenFlags.ReadWrite);

        var now = DateTimeOffset.UtcNow;
        var existing = store.Certificates
            .Find(X509FindType.FindBySubjectDistinguishedName, AgentCertificateFactory.SubjectName, false)
            .OfType<X509Certificate2>()
            .Where(certificate => certificate.HasPrivateKey)
            .Where(certificate => certificate.NotBefore.ToUniversalTime() <= now.UtcDateTime)
            .Where(certificate => certificate.NotAfter.ToUniversalTime() > now.AddDays(30).UtcDateTime)
            .OrderByDescending(certificate => certificate.NotAfter)
            .FirstOrDefault();
        if (existing is not null)
        {
            return existing;
        }

        var created = AgentCertificateFactory.CreateSelfSigned();
        store.Add(created);
        return created;
    }
}
