package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	handler := NewHandler(NewMemoryStore())

	fmt.Println("Task Manager v3 - REST API + middleware di :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
