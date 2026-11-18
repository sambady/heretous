---
name: heretous-tests
description: Prepare Heretous dependencies through Compose, start the whole product, and run all Go, PostgreSQL migration, and HTTP system tests against the running environment. Use when verifying changes or investigating build and test failures in this repository.
---

# Heretous tests

Run from the repository root. Use Docker Compose for dependency downloads and verification. Adapt Docker invocation to the current machine's instructions. Read and follow `AGENTS.md`: while its build prohibition applies, do not build images or run checks that compile code, including `go vet` and `go test`. Review source and configuration instead and report the checks that were not run.

Compose has no build definitions. The application uses the prebuilt local image `heretous:local`, which must already contain the relevant source changes. If it is missing or outdated, report that limitation without building it. The `tests` and `deps` services use the upstream `golang:1.25-bookworm` image. Tests mount the current repository read-only at `/app`; no separate test image is needed.

When dependencies change or module files need to be generated, run the `deps` service. It uses the upstream Go image, mounts the repository at `/app`, formats Go sources, and updates `go.mod` and `go.sum` using `go mod tidy`:

```sh
docker compose -f docker-compose.yml --profile tools up --exit-code-from deps deps
```

Start the whole product and wait for readiness before testing:

```sh
docker compose -f docker-compose.yml up --wait --wait-timeout 120
```

This starts PostgreSQL and the application. The server applies migrations under a PostgreSQL advisory lock before listening for HTTP; migration failure or timeout prevents startup. PostgreSQL and the application's `/readyz` health checks must pass, and application logs must confirm startup migrations completed. If startup fails, inspect `docker compose -f docker-compose.yml ps -a` and `docker compose -f docker-compose.yml logs --tail=100 app postgres` before proceeding.

Only when compilation is permitted by the current instructions, run the complete checks against that running environment:

```sh
docker compose -f docker-compose.yml run --rm -T tests
```

Runs `go vet ./...` and `go test -race -count=1 -timeout=60s ./...`. The service depends on a healthy application and a separate `postgres-test` instance. `TEST_APP_URL` points to the running application; `TEST_DATABASE_URL` points to the isolated test server. Database and migration tests create and remove their own databases there. System tests call the real application's `/healthz` and `/readyz` endpoints.

After application fixes, verification requires an updated application image. Do not build it while the build prohibition applies. When the image is available and compilation is permitted, start the whole product again, then rerun the complete checks against the current mounted test sources. A focused test run is useful for investigation but does not replace the final full run. Do not report success if integration or system tests were skipped, the product was not ready, or migrations failed.

Keep downloads and checks in Compose. Do not substitute host Go, standalone `docker run`, or mocked services for this verification. Preserve cache volumes and application data. Never run rollback tests against the application's database. Leave the running environment available unless the user asks to stop it.

Report startup readiness, migration outcome, test exit code, and any failing packages or tests. Distinguish dependency/build/startup errors from test failures.
