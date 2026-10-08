param([string]$BundleDirectory = $PSScriptRoot)
# Local portable acceptance only; fixture journals, signers and HTTPS authority.
# No real enrollment, credential installation, GUI or production TLS session.
$ErrorActionPreference = 'Stop'
$taskCases = @(
    @{Exe='owner-product-tests-win-4s.exe';Hash='79e1ea7ee686371881328e0a6751ecd3256d55d392cb98979efd832fcc153b71';Pattern='^TestOwnedProduct(Upgrade|Close|Nonzero)';Required=@('TestOwnedProductUpgradeWaitsForProductAndNetworkJoin','TestOwnedProductCloseTimeoutRetainsJournalUntilActualExit','TestOwnedProductNonzeroExitRejectsUpgrade')},
    @{Exe='runtime-product-tests-win-4s.exe';Hash='0961872609c80f6ef8b9b30ae070697502df448812e262fc87f90b15253c8f6d';Pattern='^TestProduct';Required=@('TestProductTargetPublishesPossessionAndVerifiedDescriptor','TestProductBadPossessionRejectsWithoutSecretOutput','TestProductControllerUsesVerifiedDescriptorAndPrivateConfig','TestProductDescriptorTamperRejectedBeforeProcessStart','TestProductInvalidHMACRejectedBeforeProcessStart')},
    @{Exe='signer-join-tests-win-4s.exe';Hash='37fa491ac3a0afc08a71a9e99c4603593f70affe1d0b9fe016370505de7a4843';Pattern='^TestSignerCloseJoined';Required=@('TestSignerCloseJoinedWaitsAfterClosedAck','TestSignerCloseJoinedRejectsNonzeroExit')},
    @{Exe='profile-product-tests-win-4s.exe';Hash='637731df1cce8d30760b3c2599a9b9f5067f0161aeb5929d4512653b6fa8c492';Pattern='^TestProfileCancellationDoesNotHideProductJoinFailure$';Required=@('TestProfileCancellationDoesNotHideProductJoinFailure')}
)
foreach ($taskCase in $taskCases) {
    $taskExe = Join-Path $BundleDirectory $taskCase.Exe
    if ((Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskCase.Hash) { throw 'Validation binary mismatch; use this exact bundle' }
    $taskLines = @(& $taskExe (('-test.run=' + $taskCase.Pattern)) '-test.v=true' '-test.timeout=60s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskLines | ForEach-Object { Write-Output $_ }
    $taskText = $taskLines -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '(--- SKIP:|--- FAIL:|no tests to run)') { throw 'Owned product validation failed; not accepted' }
    foreach ($taskName in $taskCase.Required) {
        if ($taskText -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($taskName) + ' [(]')) { throw 'Required assertion missing; not accepted' }
    }
}
Write-Output 'VALIDATION PASSED: owned product/journal barrier + TLS preparation fixtures + signer process join + real local HTTPS cancellation. NOT native credentials, cross-machine Headscale, actual Swift/C# TLS or installer acceptance.'
