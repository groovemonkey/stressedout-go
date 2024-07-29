# StressedOut (in Go)

## Setup

Run this binary somehow.

Export the following environment variables:

```bash
```bash
export POSTGRES_ADDR="localhost:5432"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="stressedout"
```

Run the postgres container:

```bash
docker run --rm --name pg_stressedout -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=stressedout -d -p 5432:5432 postgres
```

Connect to pg with `psql -h localhost -p 5432 -U postgres -d stressedout`

Delete with `docker kill pg_stressedout`

## Seeding Routes

- <http://localhost:8080/firstrun> - Create the database and schema
- <http://localhost:8080/seed> - Seeds the database with some data

## Testing Routes

- <http://localhost:8080/> - Static page
- <http://localhost:8080/dynamic> - Dynamic page using Go templating
- <http://localhost:8080/read> - Dynamic page that reads from the database
- <http://localhost:8080/write> - Dynamic page that writes to the database and then reads from it
