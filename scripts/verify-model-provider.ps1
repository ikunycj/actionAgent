param(
  [string]$BaseUrl = "http://127.0.0.1:8787",
  [string]$Model = "gpt-4o-mini",
  [string]$Message = "Say hello in one sentence."
)

$health = Invoke-RestMethod -Method Get -Uri "$BaseUrl/healthz"
if (-not $health.ready) {
  Write-Error "Agent is not ready: $($health | ConvertTo-Json -Depth 5)"
  exit 1
}

$body = @{
  model = $Model
  messages = @(
    @{
      role = "user"
      content = $Message
    }
  )
} | ConvertTo-Json -Depth 8

$resp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/chat/completions" -ContentType "application/json" -Body $body
$resp | ConvertTo-Json -Depth 10
