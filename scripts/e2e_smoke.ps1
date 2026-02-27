Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path "."
Set-Location $repoRoot

$dbPath = Join-Path $env:TEMP "neabrain-smoke.db"
if (Test-Path $dbPath) {
  Remove-Item $dbPath
}

function Invoke-Nea {
  param(
    [string[]]$Args
  )
  & go run ./cmd/neabrain @Args
  if ($LASTEXITCODE -ne 0) {
    throw "neabrain command failed: $Args"
  }
}

$commonArgs = @("--storage-path", $dbPath)

Invoke-Nea @("observation", "create", "--content", "Smoke test observation", "--project", "smoke", "--topic", "onboarding", "--tags", "smoke,cli") + $commonArgs
Invoke-Nea @("search", "--query", "Smoke", "--project", "smoke") + $commonArgs

$addr = "127.0.0.1:8099"
$server = Start-Process -FilePath "go" -ArgumentList @("run", "./cmd/neabrain", "serve", "--addr", $addr, "--storage-path", $dbPath) -PassThru -WindowStyle Hidden
try {
  Start-Sleep -Seconds 1
  Invoke-RestMethod -Method Get -Uri "http://$addr/observations?project=smoke" | Out-Null
} finally {
  if ($server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id
  }
}

$mcpRequest = @{
  jsonrpc = "2.0"
  id      = 1
  method  = "tools/call"
  params  = @{
    name      = "search"
    arguments = @{
      query   = "Smoke"
      project = "smoke"
    }
  }
}
$mcpJson = $mcpRequest | ConvertTo-Json -Depth 6 -Compress
$mcpResponse = $mcpJson | & go run ./cmd/neabrain mcp --storage-path $dbPath | ConvertFrom-Json
if (-not $mcpResponse.result) {
  throw "MCP search returned no result"
}

Write-Host "Smoke test completed."
