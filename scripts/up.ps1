# Start the stack (equivalent to: make up)
Set-Location $PSScriptRoot\..
docker compose -f deploy/compose/docker-compose.yml up --build
