package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func usage() {
    fmt.Fprintf(flag.CommandLine.Output(), `Usage: migrate [options] <command>
Commands:
  up            Apply all up migrations
  down          Rollback last migration
  goto <version>  Migrate to specific version
  force <version> Force set version (useful for broken state)
  version       Print current migration version
  drop          Drop everything (irreversible)

Options:
`)
    flag.PrintDefaults()
}

func main() {
    flag.Usage = usage

    // flags
    var (
        migrationsPath string
        databaseURL    string
    )
    flag.StringVar(&migrationsPath, "path", "file://../migrations", "migrations source path (file://./migrations)")
    flag.StringVar(&databaseURL, "database", os.Getenv("POSTGRES_DSN"), "database url (POSTGRES_DSN env fallback)")
    flag.Parse()

    if databaseURL == "" {
        log.Fatal("database URL is empty — set -database or POSTGRES_DSN env")
    }

    if flag.NArg() < 1 {
        usage()
        os.Exit(2)
    }

    cmd := flag.Arg(0)

    // create migrate instance (path must be like file://../migrations)
    m, err := migrate.New(migrationsPath, databaseURL)
    if err != nil {
        log.Fatalf("failed to create migrate instance: %v", err)
    }

    // ensure resources closed on exit
    defer func() {
        srcErr, dbErr := m.Close()
        if srcErr != nil {
            log.Printf("migrate source close error: %v", srcErr)
        }
        if dbErr != nil {
            log.Printf("migrate db close error: %v", dbErr)
        }
    }()

    start := time.Now()
    switch cmd {
    case "up":
        if err := m.Up(); err != nil && err != migrate.ErrNoChange {
            log.Fatalf("m.Up failed: %v", err)
        }
        log.Printf("migrations up finished in %s", time.Since(start))
    case "down":
        if err := m.Steps(-1); err != nil {
            log.Fatalf("m.Steps(-1) failed: %v", err)
        }
        log.Printf("migrations step down finished in %s", time.Since(start))
    case "goto":
        if flag.NArg() < 2 {
            log.Fatal("goto requires a version argument")
        }
        var version uint
        _, err := fmt.Sscanf(flag.Arg(1), "%d", &version)
        if err != nil {
            log.Fatalf("invalid version: %v", err)
        }
        if err := m.Migrate(uint(version)); err != nil && err != migrate.ErrNoChange {
            log.Fatalf("m.Migrate failed: %v", err)
        }
        log.Printf("migrated to version %d in %s", version, time.Since(start))
    case "force":
        if flag.NArg() < 2 {
            log.Fatal("force requires a version argument")
        }
        var version int
        _, err := fmt.Sscanf(flag.Arg(1), "%d", &version)
        if err != nil {
            log.Fatalf("invalid version: %v", err)
        }
        if err := m.Force(version); err != nil {
            log.Fatalf("m.Force failed: %v", err)
        }
        log.Printf("forced to version %d in %s", version, time.Since(start))
    case "version":
        v, dirty, err := m.Version()
        if err != nil {
            if err == migrate.ErrNilVersion {
                log.Printf("no migration applied (database empty)")
                os.Exit(0)
            }
            log.Fatalf("failed to get version: %v", err)
        }
        log.Printf("current migration version: %d (dirty=%t)", v, dirty)
    case "drop":
        if err := m.Drop(); err != nil {
            log.Fatalf("m.Drop failed: %v", err)
        }
        log.Printf("database dropped in %s", time.Since(start))
    default:
        usage()
        os.Exit(2)
    }
}
