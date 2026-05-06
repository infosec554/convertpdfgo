.PHONY: up down restart logs psql redis-cli migrate clean generate
.PHONY: build run worker tidy lint pprof asynqmon

# === Docker stack ===
up:
	docker-compose up -d

# Same stack plus the API and worker containers (production-style).
up-full:
	docker-compose --profile full up -d --build

down:
	docker-compose --profile full down

restart:
	docker-compose down
	docker-compose up -d

logs:
	docker-compose logs -f

psql:
	docker exec -it my_postgres psql -U postgres -d authservice

redis-cli:
	docker exec -it my_redis redis-cli -a 1234

migrate:
	docker exec -i my_postgres psql -U postgres -d authservice < ./migrations/postgres/0001_create_user_table.up.sql

clean:
	docker rm -f my_postgres my_redis gotenberg asynqmon || true

# === Application ===
build:
	go build -o bin/api ./cmd
	go build -o bin/worker ./cmd/worker
	go build -o bin/admin-bootstrap ./cmd/admin-bootstrap

run:
	go run ./cmd

worker:
	go run ./cmd/worker

# Admin bootstrap CLI. Examples:
#   make promote-superadmin EMAIL=zarif@example.com
#   make admin-list-permissions
#   make admin-list-roles
promote-superadmin:
	@test -n "$(EMAIL)" || (echo "EMAIL=... required" && exit 2)
	go run ./cmd/admin-bootstrap promote --email=$(EMAIL) --role=superadmin

admin-list-permissions:
	go run ./cmd/admin-bootstrap list-permissions

admin-list-roles:
	go run ./cmd/admin-bootstrap list-roles

# === Tooling ===
generate:
	swag init --g api/router.go -o api/docs

tidy:
	GOSUMDB=off go mod tidy

lint:
	go vet ./...

# === Debug ===
pprof:
	@echo "Profiles available at http://127.0.0.1:6060/debug/pprof/"
	go tool pprof http://127.0.0.1:6060/debug/pprof/heap

asynqmon:
	@echo "Asynqmon UI: http://localhost:8081"
