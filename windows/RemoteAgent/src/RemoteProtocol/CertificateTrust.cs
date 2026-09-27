using System.Security.Cryptography;

namespace RemoteProtocol;

public sealed record CertificateFingerprint
{
    public const int ByteCount = 32;
    public byte[] Bytes { get; }

    public CertificateFingerprint(byte[] bytes)
    {
        if (bytes.Length != ByteCount)
        {
            throw new ProtocolException(ProtocolError.InvalidPayload, "Certificate fingerprint length is invalid.");
        }
        Bytes = bytes.ToArray();
    }

    public static CertificateFingerprint FromCertificateDer(ReadOnlySpan<byte> certificateDer)
    {
        if (certificateDer.IsEmpty)
        {
            throw new ProtocolException(ProtocolError.InvalidPayload, "Certificate DER cannot be empty.");
        }
        return new CertificateFingerprint(SHA256.HashData(certificateDer));
    }

    public string Hexadecimal => Convert.ToHexString(Bytes).ToLowerInvariant();
}

public enum CertificateTrustDecision
{
    TrustOnFirstUse,
    Trusted,
    RejectFingerprintMismatch,
}

public sealed record CertificateTrustEvaluation(
    CertificateTrustDecision Decision,
    CertificateFingerprint PresentedFingerprint);

public static class CertificateTrustPolicy
{
    public static CertificateTrustEvaluation Evaluate(
        CertificateFingerprint? storedFingerprint,
        ReadOnlySpan<byte> certificateDer)
    {
        var presented = CertificateFingerprint.FromCertificateDer(certificateDer);
        var decision = storedFingerprint is null
            ? CertificateTrustDecision.TrustOnFirstUse
            : Authentication.ConstantTimeEquals(storedFingerprint.Bytes, presented.Bytes)
                ? CertificateTrustDecision.Trusted
                : CertificateTrustDecision.RejectFingerprintMismatch;
        return new CertificateTrustEvaluation(decision, presented);
    }
}
