$ErrorActionPreference = "Stop"
$nsisPath = Join-Path ${env:ProgramFiles(x86)} "NSIS"
$makensis = Join-Path $nsisPath "makensis.exe"
function Test-PinnedNsis {
  if (-not (Test-Path $makensis)) { return $false }
  $version = (& $makensis /VERSION | Out-String).Trim()
  return $LASTEXITCODE -eq 0 -and $version -match '^v?3\.12(?:\.0)?$'
}
if (-not (Test-PinnedNsis)) {
  choco upgrade nsis.install --version=3.12.0 --allow-downgrade --force --yes --no-progress --limit-output
  if ($LASTEXITCODE -notin @(0, 3010)) { throw "Pinned NSIS installation failed with exit code $LASTEXITCODE." }
}
if (-not (Test-PinnedNsis)) { throw "Expected NSIS 3.12.0 at $makensis." }
$env:Path = "$nsisPath;$env:Path"
$nsisPath | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append
& $makensis /VERSION
