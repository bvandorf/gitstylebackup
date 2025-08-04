<#
.SYNOPSIS
    Creates a VSS snapshot using ShadowRun.exe, mounts it to a temporary drive,
    runs the backup program, exports a registry key to a file, and cleans up.

.DESCRIPTION
    This script uses ShadowRun.exe to create a VSS snapshot of the specified volume
    and mount it as a temporary drive (default Z:). It then executes the backup program
    (gitstylebackup.exe) using the provided configuration. After the backup completes,
    the script exports a specified registry key to a file. ShadowRun.exe automatically
    cleans up (i.e. unmounts and deletes the snapshot) when done.

.PARAMETER Volume
    The volume to snapshot (default: "C:").

.PARAMETER MountDrive
    The drive letter to mount the snapshot (default: "Z").

.PARAMETER RegistryKey
    The registry key to dump (default: "HKLM\SOFTWARE").

.PARAMETER RegistryDumpOutput
    The full path of the file to which the registry dump is written.
    (Default: "RegistryDump.reg" in the script's folder)

.PARAMETER MaxRetries
    Maximum number of retries if VSS snapshot is in progress (default: 3).

.PARAMETER RetryDelaySeconds
    Delay in seconds between retries (default: 60).

.EXAMPLE
    .\run_backup.ps1 -Volume "C:" -MountDrive "Z" -RegistryKey "HKLM\SOFTWARE\MyApp"
#>

param(
    [string]$Volume = "C:",
    [string]$MountDrive = "Z",
    [string]$RegistryKey = "HKLM\SOFTWARE",
    [string]$RegistryDumpOutput,
    [int]$MaxRetries = 3,
    [int]$RetryDelaySeconds = 60
)

# Define paths relative to this script's location.
$scriptDir    = $PSScriptRoot
$BackupExe    = Join-Path $scriptDir "gitstylebackup.exe"
$ShadowRunExe = Join-Path $scriptDir "ShadowRun.exe"

# Set default for RegistryDumpOutput if not provided.
if ([string]::IsNullOrWhiteSpace($RegistryDumpOutput)) {
    $RegistryDumpOutput = Join-Path $scriptDir "Backup\RegistryDump.reg"
}

Write-Output "=== Starting ShadowRun VSS Snapshot Backup Process ==="
Write-Output "Volume to snapshot: $Volume"
Write-Output "Temporary mount drive: ${MountDrive}:"
Write-Output "Using backup executable: $BackupExe"
Write-Output "Using configuration file: $(Join-Path $scriptDir 'config.txt')"
Write-Output "Registry key to dump: $RegistryKey"
Write-Output "Registry dump output file: $RegistryDumpOutput"
Write-Output "Maximum retries: $MaxRetries"
Write-Output "Retry delay: $RetryDelaySeconds seconds"

# Prepare the backup executable parameters for ShadowRun.exe.
# We pass the backup executable via -exec and its parameters via -arg.
$execArg = "-exec=`"$BackupExe`""
$arg1    = "-arg=-b"
$arg2    = "-arg=-c"
$arg3    = "-arg=`"$(Join-Path $scriptDir 'config.txt')`""

Write-Output "Backup command parameters: $arg1 $arg2 $arg3"

# Construct the arguments array for ShadowRun.exe.
$shadowRunArgs = @(
    "-mount",
    "-drive=$MountDrive",
    $execArg,
    $arg1,
    $arg2,
    $arg3,
    $Volume
)

# Initialize retry counter
$retryCount = 0
$backupExitCode = -1
$success = $false

# Retry loop for VSS snapshot
while (-not $success -and $retryCount -le $MaxRetries) {
    if ($retryCount -gt 0) {
        Write-Output "Retry attempt $retryCount of $MaxRetries after waiting $RetryDelaySeconds seconds..."
    }
    
    Write-Output "Executing ShadowRun.exe with arguments:"
    Write-Output ($shadowRunArgs -join " ")

    # Execute ShadowRun.exe with the specified arguments.
    & $ShadowRunExe @shadowRunArgs

    $backupExitCode = $LASTEXITCODE
    Write-Output "ShadowRun.exe exited with code: $backupExitCode"

    # Check if it was successful or if we need to retry
    if ($backupExitCode -eq 0) {
        $success = $true
    }
    elseif ($backupExitCode -eq 2 -and $retryCount -lt $MaxRetries) {
        # Exit code 2 often indicates VSS_E_SNAPSHOT_SET_IN_PROGRESS
        Write-Output "VSS snapshot is in progress by another process. Waiting before retry..."
        Start-Sleep -Seconds $RetryDelaySeconds
        $retryCount++
    }
    else {
        # Other error or max retries reached
        break
    }
}

# Check if ShadowRun.exe was successful after all retries
if ($backupExitCode -ne 0) {
    Write-Error "ShadowRun.exe failed with exit code $backupExitCode after $retryCount retries. Aborting backup process."
    Write-Output "=== Backup Process Failed ==="
    exit $backupExitCode
}

# --- Registry Dump Section ---
Write-Output "=== Starting Registry Dump ==="
Write-Output "Exporting registry key '$RegistryKey' to '$RegistryDumpOutput'..."

# Use reg.exe to export the registry key. The /y flag forces overwrite.
$regDump = & reg.exe export "$RegistryKey" "$RegistryDumpOutput" /y

if ($LASTEXITCODE -eq 0) {
    Write-Output "Registry dump completed successfully."
} else {
    Write-Error "Registry dump encountered errors."
}

Write-Output "=== Backup Process Completed ==="
exit $backupExitCode
