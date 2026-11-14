---
name: heretous-tests
description: Run Heretous Go checks in Docker when verifying changes or investigating test failures.
---

# Heretous tests

Run from the repository root:

```sh
docker compose -f docker-compose.yml run --build --rm -T tests
```

Runs `go vet ./...` and `go test -race -count=1 -timeout=60s ./...`.

After fixes, rerun the full command. Report the exit code and failures, distinguishing Docker/build errors from test failures. Keep checks in Docker and preserve cache volumes.
