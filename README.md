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
docker run --rm --name pg_stressedout \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=stressedout \
  # this way of setting max connections doesn't work
  # -e POSTGRES_MAX_CONNECTIONS=1000 \
  # but using -N max_conns does
  -d -p 5432:5432 postgres -N 400
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

## Perf Testing

```bash
cd perf
./wrk.sh`
```

## TODO

- There's a bug in the /write handler where the user sometimes seems to be nil. In the logs, it looks like this: `2024/07/29 17:15:53 error inserting new review: ERROR #23502 null value in column "product_id" of relation "reviews" violates not-null constraint` (happens when attempting to insert orders, too)
