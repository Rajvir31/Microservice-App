# Generate Go code from proto files. Run from repo root.
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location (Join-Path $Root "proto")
docker run --rm -v "${Root}:/workspace" -w /workspace/proto bufbuild/buf:latest generate
Write-Host "Generated code in $Root\gen"
