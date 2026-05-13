// admin-bootstrap is a one-shot CLI for first-time admin setup.
//
//	go run ./cmd/admin-bootstrap promote --email=zarif@example.com --role=admin
//	go run ./cmd/admin-bootstrap list-permissions
//	go run ./cmd/admin-bootstrap list-roles
//
// All operations connect with the same DB env as the API (`config.Load()`).
// The tool refuses to run unless the `permissions` table exists — so if
// you forgot `make migrate`, you'll see a clear error.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/config"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	cfg := config.Load()
	pool, err := connect(cfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ensureRBACMigrated(ctx, pool); err != nil {
		log.Fatalf("RBAC tables not found — run `make migrate` first: %v", err)
	}

	switch cmd {
	case "promote":
		runPromote(ctx, pool, args)
	case "list-permissions":
		runListPermissions(ctx, pool)
	case "list-roles":
		runListRoles(ctx, pool)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `convertpdfgo admin-bootstrap

Commands:
  promote --email=<EMAIL> --role=<ROLE>
      Set the role of an existing user. ROLE in {user, admin}.
  list-permissions
      Print every permission row from the permissions catalog.
  list-roles
      Print which roles have which permissions.
  help
      Show this message.

Environment:
  Reads the same .env / POSTGRES_* env vars as the main API.
`)
}

func connect(cfg config.Config) (*pgxpool.Pool, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword,
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB,
	)
	return pgxpool.New(context.Background(), url)
}

func ensureRBACMigrated(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'permissions')`
	var ok bool
	if err := pool.QueryRow(ctx, q).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("permissions table missing")
	}
	return nil
}

// ---- promote ----

func runPromote(ctx context.Context, pool *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	email := fs.String("email", "", "email of the existing user (required)")
	role := fs.String("role", "", "new role: user | admin (required)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("flags: %v", err)
	}
	if *email == "" || *role == "" {
		fs.Usage()
		os.Exit(2)
	}
	switch strings.ToLower(*role) {
	case "user", "admin":
	default:
		log.Fatalf("invalid role: %q", *role)
	}

	tag, err := pool.Exec(ctx,
		`UPDATE users SET role = $2 WHERE email = $1 AND status = 'active'`,
		*email, strings.ToLower(*role),
	)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	if tag.RowsAffected() == 0 {
		log.Fatalf("no active user found for email %q (did they register?)", *email)
	}
	fmt.Printf("✓ %s → role=%s\n", *email, *role)
	fmt.Println("Tip: any tokens issued before this change still carry the old role; ask the user to re-login.")
}

// ---- list-permissions ----

func runListPermissions(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `SELECT name, COALESCE(description,'') FROM permissions ORDER BY name`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	fmt.Printf("%-22s  %s\n", "PERMISSION", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 60))
	for rows.Next() {
		var name, desc string
		if err := rows.Scan(&name, &desc); err != nil {
			continue
		}
		fmt.Printf("%-22s  %s\n", name, desc)
	}
}

// ---- list-roles ----

func runListRoles(ctx context.Context, pool *pgxpool.Pool) {
	const q = `
		SELECT rp.role, COALESCE(string_agg(p.name, ', ' ORDER BY p.name), '')
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		GROUP BY rp.role
		ORDER BY rp.role
	`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	fmt.Printf("%-12s  %s\n", "ROLE", "PERMISSIONS")
	fmt.Println(strings.Repeat("-", 80))
	for rows.Next() {
		var role, perms string
		if err := rows.Scan(&role, &perms); err != nil {
			continue
		}
		fmt.Printf("%-12s  %s\n", role, perms)
	}
	if rows.Err() == pgx.ErrNoRows {
		fmt.Println("(no roles configured — re-run migrations to seed)")
	}
}
