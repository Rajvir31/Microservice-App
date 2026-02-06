# Running on Windows (without Make)

Use these from **PowerShell** or **Command Prompt**. Run from the **reliability-lab** folder (this folder).

## Start the stack

```powershell
cd "C:\Users\Rajvir\Desktop\Microservice App\reliability-lab"
docker compose -f deploy/compose/docker-compose.yml up --build
```

Or run the script:

```powershell
cd "C:\Users\Rajvir\Desktop\Microservice App\reliability-lab"
.\scripts\up.ps1
```

To run in the background instead of attaching to logs, add `-d`:

```powershell
docker compose -f deploy/compose/docker-compose.yml up --build -d
```

## Stop and remove everything

```powershell
cd "C:\Users\Rajvir\Desktop\Microservice App\reliability-lab"
docker compose -f deploy/compose/docker-compose.yml down -v
```

Or:

```powershell
.\scripts\down.ps1
```

## Test the API (demo)

After the stack is up:

```powershell
.\scripts\demo.ps1
```

Or manually:

```powershell
# Create order
Invoke-RestMethod -Uri "http://localhost:8080/orders" -Method Post -Body '{"user_id":"u123","amount_cents":1299,"currency":"USD","idempotency_key":"demo-123"}' -ContentType "application/json"

# Get order (replace ORDER_ID with the id from above)
Invoke-RestMethod -Uri "http://localhost:8080/orders/ORDER_ID" -Method Get
```

## View gateway logs

```powershell
docker compose -f deploy/compose/docker-compose.yml logs -f gateway
```

## Check containers

```powershell
docker compose -f deploy/compose/docker-compose.yml ps
```

## URLs

- **Grafana:** http://localhost:3000 (admin / admin)
- **Prometheus:** http://localhost:9090
- **Gateway:** http://localhost:8080

---

**Note:** You need **Docker Desktop** (or Docker Engine) installed. If `docker` is not recognized, install Docker Desktop for Windows and ensure it is running.
