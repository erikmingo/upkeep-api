# upkeep-api

```sh
go run ./cmd/api            # http://localhost:8080/health  (PORT=8099 to change)
go test ./... && go vet ./...
docker build -t upkeep-api . && docker run --rm -p 8080:8080 upkeep-api
```

From a phone on the same Wi-Fi: `http://$(ipconfig getifaddr en0):8080/health`. Port 8080 is often taken by a local Traefik on this machine, so run with `PORT=8099`.
