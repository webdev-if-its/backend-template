package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// dsnDefault cocok dengan docker-compose.yml yang kalian tulis di Level 10.
const dsnDefault = "postgres://taskmanager:rahasia@localhost:5433/taskmanager?sslmode=disable"

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = dsnDefault
	}

	db, err := Connect(dsn)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v\n(sudah menjalankan `docker compose up -d --wait`?)", err)
	}
	defer db.Close()

	store := NewPostgresStore(db)
	if err := store.Migrate(); err != nil {
		log.Fatalf("migrasi gagal: %v", err)
	}

	fmt.Println("Task Manager v3 - REST API di atas PostgreSQL, di :8080")
	log.Fatal(http.ListenAndServe(":8080", NewAPI(store).Routes()))
}
