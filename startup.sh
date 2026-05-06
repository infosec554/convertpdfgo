#!/bin/bash
echo "Waiting for Docker containers to be ready..."
while ! docker ps | grep my_postgres; do
  sleep 5
done

echo "Waiting for PostgreSQL to accept connections..."
until docker exec -i my_postgres psql -U postgres -c "SELECT 1" > /dev/null 2>&1; do
  sleep 3
done

echo "Applying Migrations..."
docker exec -i my_postgres psql -U postgres -d authservice < ./migrations/postgres/0001_create_user_table.up.sql
docker exec -i my_postgres psql -U postgres -d authservice < ./migrations/postgres/0002_alter_users_password.up.sql

echo "Starting PDFNinja API..."
go run cmd/main.go
