# なでしこ版DocTestを、配置したgonakoで起動する。
$ErrorActionPreference = 'Stop'
$runtime = $env:GONAKO_RUNTIME
if (-not $runtime) {
    $runtime = Join-Path $PSScriptRoot 'gonako.exe'
    if (-not (Test-Path $runtime)) {
        $runtime = (Get-Command gonako -ErrorAction Stop).Source
    }
}
$previousRuntime = $env:GONAKO_DOCTEST_RUNTIME
try {
    $env:GONAKO_DOCTEST_RUNTIME = $runtime
    $source = Join-Path (Split-Path $PSScriptRoot -Parent) 'gonako-package/doctest.nako3'
    & $runtime $source @args
    $code = $LASTEXITCODE
} finally {
    $env:GONAKO_DOCTEST_RUNTIME = $previousRuntime
}
exit $code
