# upkeep-api

```sh
cp .env.example .env     # PORT, DATABASE_URL
make db                  # Postgres 17 on localhost:5433 (5432 is usually taken by the TFA env)
make run                 # http://localhost:8099/health
make test && make lint
docker build -t upkeep-api . && docker run --rm -p 8080:8080 -e DATABASE_URL=... upkeep-api
```

From a phone on the same Wi-Fi: `http://$(ipconfig getifaddr en0):8099/health`.
