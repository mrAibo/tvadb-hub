# Unit-like tests for the pure release-authorization gate.
#
# These tests never call `gh`, never publish, never dispatch a workflow and never
# contact the network. They exercise the same function the Windows Release
# workflow dot-sources, so a policy change cannot silently diverge from the tests.
#
# Run: pwsh -NoProfile -File scripts/new-release-authorization-helper/tests/ReleaseAuthorization.Tests.ps1

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..' 'Resolve-ReleaseAuthorization.ps1')

$script:passed = 0
$script:failed = 0

$Version = '0.1.0'
# GitHub currently supplies full SHA-1 commit IDs, not SHA-256 artifact digests.
$GoodShaA = '68c6cc62667494bcce1a4e747f0cbf05783287c8'
$GoodShaB = 'b' * 40

function Invoke-Case {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][scriptblock]$Body
    )

    try {
        & $Body
        $script:passed++
        Write-Host "PASS  $Name"
    }
    catch {
        $script:failed++
        Write-Host "FAIL  $Name"
        Write-Host "      $($_.Exception.Message)"
    }
}

function Assert-Equal {
    param($Expected, $Actual, [string]$Because = '')

    if ($Expected -ne $Actual) {
        throw "expected '$Expected' but got '$Actual' $Because"
    }
}

function Assert-Throws {
    param([Parameter(Mandatory)][scriptblock]$Body, [string]$Match = '')

    $threw = $false
    try {
        & $Body | Out-Null
    }
    catch {
        $threw = $true
        if ($Match -and $_.Exception.Message -notmatch $Match) {
            throw "threw with unexpected message '$($_.Exception.Message)' (wanted /$Match/)"
        }
    }
    if (-not $threw) {
        throw 'expected a fail-closed error but the call succeeded'
    }
}

function Resolve-Gate {
    param(
        [string]$EventName = 'workflow_dispatch',
        [string]$RefName = '',
        [string]$PublishRequested = 'false',
        [string]$PhysicalDeviceValidatedInput = 'false',
        [string]$ValidatedVersion = '',
        [string]$ValidatedSha = '',
        [string]$ProjectVersion = $Version,
        [string]$CheckoutSha = $GoodShaA
    )

    Resolve-ReleaseAuthorization -EventName $EventName -RefName $RefName -PublishRequested $PublishRequested `
        -PhysicalDeviceValidatedInput $PhysicalDeviceValidatedInput -ValidatedVersion $ValidatedVersion `
        -ValidatedSha $ValidatedSha -ProjectVersion $ProjectVersion -CheckoutSha $CheckoutSha
}

Write-Host 'Release authorization helper tests'

# --- SHA shape -------------------------------------------------------------

Invoke-Case 'Test-ReleaseSourceSha accepts full SHA-1 and SHA-256 Git object IDs' {
    Assert-Equal $true (Test-ReleaseSourceSha -Value $GoodShaA)
    Assert-Equal $true (Test-ReleaseSourceSha -Value ($GoodShaA.ToUpperInvariant()))
    Assert-Equal $true (Test-ReleaseSourceSha -Value ('c' * 64))
}

Invoke-Case 'Test-ReleaseSourceSha rejects truncated, extended, non-hex and empty IDs' {
    foreach ($length in @(39, 41, 63, 65)) {
        Assert-Equal $false (Test-ReleaseSourceSha -Value ('a' * $length))
    }
    Assert-Equal $false (Test-ReleaseSourceSha -Value ('z' * 40))
    Assert-Equal $false (Test-ReleaseSourceSha -Value ('z' * 64))
    Assert-Equal $false (Test-ReleaseSourceSha -Value '')
}

# --- Unpublished candidate builds ------------------------------------------

Invoke-Case 'manual run with publish=false builds without any gate variables' {
    $result = Resolve-Gate
    Assert-Equal $Version $result.Version
    Assert-Equal "v$Version" $result.Tag
    Assert-Equal $false $result.Publishing
    Assert-Equal '' $result.AuthorizedSha
}

Invoke-Case 'manual run with publish=false stays unpublished even with gate variables set' {
    $result = Resolve-Gate -PublishRequested 'false' -ValidatedVersion $Version -ValidatedSha $GoodShaA -PhysicalDeviceValidatedInput 'true'
    Assert-Equal $false $result.Publishing
}

# --- Tag publishing --------------------------------------------------------

Invoke-Case 'tag push with matching version and source SHA publishes' {
    $result = Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version -ValidatedSha $GoodShaA
    Assert-Equal $true $result.Publishing
    Assert-Equal "v$Version" $result.Tag
    Assert-Equal $GoodShaA $result.AuthorizedSha
}

Invoke-Case 'tag push accepts an uppercase gate SHA and normalizes it' {
    $result = Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version -ValidatedSha ($GoodShaA.ToUpperInvariant())
    Assert-Equal $GoodShaA $result.AuthorizedSha
}

Invoke-Case 'tag push with a mismatched tag fails closed' {
    Assert-Throws -Match 'does not match project version' -Body { Resolve-Gate -EventName 'push' -RefName 'v0.2.0' -ValidatedVersion $Version -ValidatedSha $GoodShaA }
}

Invoke-Case 'tag push with a missing validated version fails closed' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_VERSION' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedSha $GoodShaA }
}

Invoke-Case 'tag push with a wrong validated version fails closed' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_VERSION' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion '0.0.9' -ValidatedSha $GoodShaA }
}

Invoke-Case 'tag push with a missing validated SHA fails closed' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_SHA' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version }
}

Invoke-Case 'tag push with a malformed validated SHA fails closed' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_SHA' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version -ValidatedSha ($GoodShaA.Substring(0, 39)) }
}

Invoke-Case 'tag push with a valid SHA of another commit of the same version fails closed' {
    Assert-Throws -Match 'authorized for source' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version -ValidatedSha $GoodShaB -CheckoutSha $GoodShaA }
}

Invoke-Case 'a SHA-256 artifact digest cannot authorize a different SHA-1 commit' {
    Assert-Throws -Match 'authorized for source' -Body {
        Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version `
            -ValidatedSha ('c' * 64) -CheckoutSha $GoodShaA
    }
}

Invoke-Case 'tag push with a non-hex checkout SHA fails closed' {
    Assert-Throws -Match 'checked-out commit SHA' -Body { Resolve-Gate -EventName 'push' -RefName "v$Version" -ValidatedVersion $Version -ValidatedSha $GoodShaA -CheckoutSha 'deadbeef' }
}

# --- Manual publishing -----------------------------------------------------

Invoke-Case 'manual publish with matching version, source SHA and consent publishes' {
    $result = Resolve-Gate -PublishRequested 'true' -ValidatedVersion $Version -ValidatedSha $GoodShaA -PhysicalDeviceValidatedInput 'true'
    Assert-Equal $true $result.Publishing
    Assert-Equal $GoodShaA $result.AuthorizedSha
}

Invoke-Case 'manual publish without consent fails closed' {
    Assert-Throws -Match 'physical_device_validated=true' -Body { Resolve-Gate -PublishRequested 'true' -ValidatedVersion $Version -ValidatedSha $GoodShaA }
}

Invoke-Case 'manual publish with a missing validated SHA fails even with consent' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_SHA' -Body { Resolve-Gate -PublishRequested 'true' -ValidatedVersion $Version -PhysicalDeviceValidatedInput 'true' }
}

Invoke-Case 'manual publish with a wrong validated SHA fails closed' {
    Assert-Throws -Match 'authorized for source' -Body { Resolve-Gate -PublishRequested 'true' -ValidatedVersion $Version -ValidatedSha $GoodShaB -PhysicalDeviceValidatedInput 'true' }
}

Invoke-Case 'manual publish with a wrong validated version fails closed' {
    Assert-Throws -Match 'PHYSICAL_DEVICE_VALIDATED_VERSION' -Body { Resolve-Gate -PublishRequested 'true' -ValidatedVersion '0.0.9' -ValidatedSha $GoodShaA -PhysicalDeviceValidatedInput 'true' }
}

# --- Version shape ---------------------------------------------------------

Invoke-Case 'a non-stable project version fails closed' {
    Assert-Throws -Match 'stable three-part version' -Body { Resolve-Gate -ProjectVersion '0.1' }
}

Write-Host ''
Write-Host "passed=$($script:passed) failed=$($script:failed)"
if ($script:failed -ne 0) {
    exit 1
}
exit 0
