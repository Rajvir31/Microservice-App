# Stop and remove containers/volumes (equivalent to: make down)
Set-Location $PSScriptRoot\..
docker compose -f deploy/compose/docker-compose.yml down -v
