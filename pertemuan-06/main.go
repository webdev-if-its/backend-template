package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	api := NewAPI(NewMemoryStore())

	fmt.Println("Task Manager v2 - REST API di :8080")
	log.Fatal(http.ListenAndServe(":8080", api.Routes()))
}
