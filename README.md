# Job Application Tracker API

REST API for tracking job applications. Built with Go, Chi router, and SQLite.

## Build

```bash
go build -o tracker ./cmd/server
```

## Run

```bash
./tracker
```

Server starts on `localhost:8081`. Override with `PORT` env var. Database created automatically at `./data/tracker.db`.

## API

### List applications

```bash
curl http://localhost:8081/applications

# Filter by status
curl http://localhost:8081/applications?status=applied
```

### Create application

```bash
curl -X POST http://localhost:8081/applications \
  -H 'Content-Type: application/json' \
  -d '{
    "company": "Acme Corp",
    "role": "Senior Backend Engineer",
    "url": "https://acme.com/careers/123",
    "salary_min": 150000,
    "salary_max": 200000,
    "location": "Remote",
    "status": "applied",
    "notes": "Referred by Jane",
    "applied_at": "2026-02-09"
  }'
```

### Get application

```bash
curl http://localhost:8081/applications/{id}
```

### Update application

```bash
curl -X PUT http://localhost:8081/applications/{id} \
  -H 'Content-Type: application/json' \
  -d '{"status": "interview", "notes": "Phone screen scheduled"}'
```

### Delete application

```bash
curl -X DELETE http://localhost:8081/applications/{id}
```

## Status Values

`wishlist`, `applied`, `phone_screen`, `interview`, `offer`, `accepted`, `rejected`, `withdrawn`, `ghosted`

Default on create: `wishlist`

## Test

```bash
go test ./... -v
```
