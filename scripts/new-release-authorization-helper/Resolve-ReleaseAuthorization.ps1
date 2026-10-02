# Pure release-authorization resolver for the Windows Release workflow.
#
# Dot-source this file and call Resolve-ReleaseAuthorization. The function is
# intentionally pure: it reads no environment variables, touches no files, makes
# no network calls and writes no output. The workflow passes every input
# explicitly, so scripts/new-release-authorization-helper/tests exercises exactly
# the logic that runs in CI.
#
# Publication policy (see docs/RELEASING.md):
#   * publishing = a `v*` tag push, or a manual run with publish=true;
#   * publishing requires repository variable PHYSICAL_DEVICE_VALIDATED_VERSION
#     to equal the project version;
#   * publishing requires repository variable PHYSICAL_DEVICE_VALIDATED_SHA to be
#     an exact 40- or 64-character hexadecimal commit SHA equal to the checked-out
#     github.sha, for BOTH tag and manual publishing;
#   * manual publishing additionally requires the physical_device_validated input;
#   * an unpublished candidate build needs no gate variables at all;
#   * every failure path throws, i.e. fails closed.

function Test-ReleaseSourceSha {
    [CmdletBinding()]
    [OutputType([bool])]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Value
    )

    return [bool]($Value -match '^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$')
}

function Resolve-ReleaseAuthorization {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$EventName,

        [AllowEmptyString()]
        [string]$RefName = '',

        [AllowEmptyString()]
        [string]$PublishRequested = 'false',

        [AllowEmptyString()]
        [string]$PhysicalDeviceValidatedInput = 'false',

        [AllowEmptyString()]
        [string]$ValidatedVersion = '',

        [AllowEmptyString()]
        [string]$ValidatedSha = '',

        # Project version read from Makefile/build config/in-app version.
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$ProjectVersion,

        # github.sha of the commit actually checked out for this run.
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$CheckoutSha
    )

    if ([string]::IsNullOrWhiteSpace($ProjectVersion) -or $ProjectVersion -notmatch '^\d+\.\d+\.\d+$') {
        throw "Project version must be a stable three-part version (for example 0.1.0). Got '$ProjectVersion'."
    }

    $tag = "v$ProjectVersion"
    $isTagRun = $EventName -eq 'push'
    $isManualRun = $EventName -eq 'workflow_dispatch'

    if ($isTagRun -and $RefName -ne $tag) {
        throw "Tag '$RefName' does not match project version '$tag'."
    }

    $publishing = $isTagRun -or ($isManualRun -and $PublishRequested -eq 'true')
    $authorizedSha = ''

    if ($publishing) {
        if ($ValidatedVersion -ne $ProjectVersion) {
            throw "Publishing $tag requires repository variable PHYSICAL_DEVICE_VALIDATED_VERSION='$ProjectVersion'. Current value: '$ValidatedVersion'."
        }

        if (-not (Test-ReleaseSourceSha -Value $ValidatedSha)) {
            throw "Publishing $tag requires repository variable PHYSICAL_DEVICE_VALIDATED_SHA to be an exact 40- or 64-character hexadecimal commit SHA. Current value: '$ValidatedSha'."
        }

        if (-not (Test-ReleaseSourceSha -Value $CheckoutSha)) {
            throw "The checked-out commit SHA '$CheckoutSha' is not an exact 40- or 64-character hexadecimal value; refusing to publish $tag."
        }

        $authorizedSha = $ValidatedSha.ToLowerInvariant()
        $checkedOutSha = $CheckoutSha.ToLowerInvariant()
        if ($authorizedSha -ne $checkedOutSha) {
            throw "Publishing $tag is authorized for source $authorizedSha but this run checked out $checkedOutSha. Refusing to publish a different source commit for the same version."
        }

        if ($isManualRun -and $PhysicalDeviceValidatedInput -ne 'true') {
            throw "Manual publishing additionally requires physical_device_validated=true."
        }
    }

    return [pscustomobject]@{
        Version         = $ProjectVersion
        Tag             = $tag
        Publishing      = $publishing
        AuthorizedSha   = $authorizedSha
    }
}
