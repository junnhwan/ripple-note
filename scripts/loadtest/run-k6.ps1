param(
    [ValidateSet("latest-anonymous", "hot-anonymous", "latest-auth", "mixed")]
    [string]$Scenario = "latest-anonymous",

    [string]$BaseUrl = "http://127.0.0.1:18080",
    [int]$Vus = 100,
    [string]$Duration = "2m",
    [int]$Limit = 20,
    [double]$Sleep = 0,
    [string]$LoginEmail = "loadtest@ripple.dev",
    [string]$LoginPassword = "loadtest123",
    [string]$OutputDir = "reports/loadtest"
)

$ErrorActionPreference = "Stop"

$scriptByScenario = @{
    "latest-anonymous" = "feed_latest_anonymous.js"
    "hot-anonymous"    = "feed_hot_anonymous.js"
    "latest-auth"      = "feed_latest_auth.js"
    "mixed"            = "feed_mixed.js"
}

$scriptName = $scriptByScenario[$Scenario]
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$loadtestDir = Join-Path $repoRoot "scripts\loadtest"
$outputPath = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Force -Path $outputPath | Out-Null

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$summaryFile = Join-Path $outputPath "${Scenario}_${Vus}vu_${Duration}_${timestamp}.txt"

$volumePath = $loadtestDir -replace "\\", "/"
if ($IsWindows -or $env:OS -eq "Windows_NT") {
    $volumePath = $volumePath -replace "^([A-Za-z]):", '/$1'
}

$envArgs = @(
    "-e", "BASE_URL=$BaseUrl",
    "-e", "VUS=$Vus",
    "-e", "DURATION=$Duration",
    "-e", "LIMIT=$Limit",
    "-e", "SLEEP=$Sleep",
    "-e", "LOGIN_EMAIL=$LoginEmail",
    "-e", "LOGIN_PASSWORD=$LoginPassword"
)

$k6Args = @(
    "run",
    "--quiet",
    "--summary-trend-stats", "avg,min,med,p(90),p(95),p(99),max",
    "/scripts/$scriptName"
)

Write-Host "Running k6 scenario: $Scenario"
Write-Host "Base URL: $BaseUrl"
Write-Host "Output: $summaryFile"

$output = & docker run --rm @envArgs -v "${volumePath}:/scripts" grafana/k6 @k6Args 2>&1
$exitCode = $LASTEXITCODE
$output | Tee-Object -FilePath $summaryFile

if ($exitCode -ne 0) {
    throw "k6 scenario '$Scenario' failed with exit code $exitCode"
}
