# ElaMachan developer Makefile.
# Wraps common local-dev tasks. Migrations run via the migrate/migrate Docker image
# so contributors do not need a local golang-migrate install.

MIGRATE_VERSION ?= v4.17.1
MIGRATIONS_DIR  ?= backend/migrations
# Default to the local docker-compose Postgres. Override DATABASE_URL for other targets.
DATABASE_URL    ?= postgres://elamachan:elamachan@localhost:5432/elamachan?sslmode=disable

# Run golang-migrate in Docker, on the host network so it can reach localhost:5432.
MIGRATE = docker run --rm --network host \
	-v $(CURDIR)/$(MIGRATIONS_DIR):/migrations \
	migrate/migrate:$(MIGRATE_VERSION) \
	-path=/migrations -database "$(DATABASE_URL)"

.PHONY: up down migrate-up migrate-down migrate-create test build help

help:
	@echo "Targets:"
	@echo "  up             - docker compose up -d (Postgres, Redis, Meilisearch)"
	@echo "  down           - docker compose down -v"
	@echo "  migrate-up     - apply all pending migrations"
	@echo "  migrate-down   - roll back the last migration"
	@echo "  migrate-create name=foo - scaffold a new migration pair"
	@echo "  build          - go build ./... (backend)"
	@echo "  test           - go test ./... (backend)"

up:
	docker compose up -d

down:
	docker compose down -v

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=<migration_name>" && exit 1)
	docker run --rm -v $(CURDIR)/$(MIGRATIONS_DIR):/migrations \
		migrate/migrate:$(MIGRATE_VERSION) \
		create -ext sql -dir /migrations -seq $(name)

build:
	cd backend && go build ./...

test:
	cd backend && go test ./...
