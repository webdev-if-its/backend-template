package main

// api.go -- REST API dari Pertemuan 6 (routing, JSON, error terstruktur) yang
// sudah berjalan. Di tugas ini hampir semuanya TIDAK berubah -- itulah gunanya
// interface TaskStore. Satu-satunya yang perlu kalian sentuh adalah listTasks
// (TODO Level 10), karena List() sekarang bisa mengembalikan error.

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type taskInput struct {
	Judul   string `json:"judul"`
	Selesai bool   `json:"selesai"`
}

type errorBody struct {
	Error string `json:"error"`
}

type API struct {
	store TaskStore
}

func NewAPI(store TaskStore) *API {
	return &API{store: store}
}

func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", a.listTasks)
	mux.HandleFunc("POST /tasks", a.createTask)
	mux.HandleFunc("GET /tasks/{id}", a.getTask)
	mux.HandleFunc("PUT /tasks/{id}", a.updateTask)
	mux.HandleFunc("DELETE /tasks/{id}", a.deleteTask)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTugasTidakDitemukan):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "tugas tidak ditemukan"})
	case errors.Is(err, ErrInputKosong):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "input tidak boleh kosong"})
	default:
		log.Printf("error internal: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "terjadi kesalahan internal"})
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "id harus berupa angka"})
		return 0, false
	}
	return id, true
}

func decodeInput(w http.ResponseWriter, r *http.Request) (taskInput, bool) {
	var in taskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "body bukan JSON yang valid"})
		return in, false
	}
	return in, true
}

// TODO(Level 10): List() sekarang ([]Task, error). Baris di bawah membuang
// error-nya dengan `_` -- ganti dengan penanganan yang benar (teruskan ke
// writeError supaya klien mendapat 500 generik, bukan 200 dengan data kosong).
func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	tugas, _ := a.store.List()
	writeJSON(w, http.StatusOK, tugas)
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	t, err := a.store.Add(Task{Judul: in.Judul, Selesai: in.Selesai})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := a.store.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *API) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	t, err := a.store.Update(Task{ID: id, Judul: in.Judul, Selesai: in.Selesai})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *API) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := a.store.Delete(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
