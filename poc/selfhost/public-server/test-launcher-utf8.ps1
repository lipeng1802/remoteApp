# Developer regression: execute only the launcher's trusted UTF-8 writer,
# never its broker/credential/bootstrap lifecycle. No remote file writes.
param([string]$Launcher = (Join-Path $PSScriptRoot 'start-interactive-vault.ps1'), [string]$LauncherSource = '')
$ErrorActionPreference = 'Stop'
$taskTokens = $null
$taskErrors = $null
if (!$LauncherSource) { $LauncherSource = [System.IO.File]::ReadAllText($Launcher) }
$taskAst = [System.Management.Automation.Language.Parser]::ParseInput($LauncherSource, [ref]$taskTokens, [ref]$taskErrors)
if ($taskErrors.Count -ne 0) { throw 'Launcher syntax invalid' }
$taskFunction = $taskAst.Find({ param($a) $a -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $a.Name -eq 'Write-VaultControlLine' }, $true)
if (!$taskFunction) { throw 'UTF-8 writer missing' }
Invoke-Expression $taskFunction.Extent.Text
$taskPath = 'H:\' + [char]0x8fdc + [char]0x7a0b + '\' + [char]0x6d4b + [char]0x8bd5 + ' ' + [char]0x8f6f + [char]0x4ef6 + '\identity-vault.exe'
$taskLine = @{ VaultExecutable=$taskPath } | ConvertTo-Json -Compress
$taskLength = [System.Text.Encoding]::UTF8.GetByteCount($taskLine + "`n")
$taskChild = '$taskLength=' + $taskLength + ';' + {
    $taskInput = [Console]::OpenStandardInput()
    $taskMemory = New-Object System.IO.MemoryStream
    for ($i=0; $i -lt $taskLength; $i++) {
        $taskByte = $taskInput.ReadByte()
        if ($taskByte -lt 0) { throw 'Short input' }
        $taskMemory.WriteByte([byte]$taskByte)
    }
    [Console]::WriteLine([Convert]::ToBase64String($taskMemory.ToArray()))
}.ToString()
$taskInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskInfo.FileName = (Get-Process -Id $PID).Path
$taskInfo.Arguments = '-NoProfile -EncodedCommand ' + [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($taskChild))
$taskInfo.UseShellExecute = $false
$taskInfo.RedirectStandardInput = $true
$taskInfo.RedirectStandardOutput = $true
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = $taskInfo
$taskStarted = $false
try {
    $taskStarted = $taskProcess.Start()
    if (!$taskStarted) { throw 'Regression child launch failed' }
    # Non-ASCII directory plus a space; source remains ASCII for PS 5.1.
    Write-VaultControlLine -Stream $taskProcess.StandardInput.BaseStream -Line $taskLine
    $taskProcess.StandardInput.Close()
    $taskRead = $taskProcess.StandardOutput.ReadToEndAsync()
    if (!$taskRead.Wait(5000) -or !$taskProcess.WaitForExit(5000)) { throw 'Regression child timeout' }
    $taskExpected = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($taskLine + "`n"))
    if ($taskProcess.ExitCode -ne 0 -or $taskRead.Result.Trim() -ne $taskExpected) { throw 'Chinese JSON UTF-8 roundtrip failed' }
    Write-Output 'UTF8_PROCESS_ROUNDTRIP_PASS'
} finally {
    if ($taskStarted -and !$taskProcess.HasExited) { $taskProcess.Kill(); $taskProcess.WaitForExit() }
    $taskProcess.Dispose()
}
