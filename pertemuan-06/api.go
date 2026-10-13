package main

import (
	"net/http"
)

// TODO: lihat SOAL.md untuk kontrak lengkap. Ganti setiap "panic" dengan
// implementasi yang benar; tambahkan import (encoding/json, errors, log,
// strconv, ...) sendiri kalau dibutuhkan.

// taskInput adalah bentuk JSON yang boleh dikirim klien untuk POST/PUT.
// Sengaja TIDAK memuat ID -- ID bukan urusan klien.
type taskInput struct {
	Judul   string `json:"judul"`
	Selesai bool   `json:"selesai"`
}

// errorBody adalah bentuk JSON semua respons error: {"error":"pesan"}.
type errorBody struct {
	Error string `json:"error"`
}

// API menyatukan routing dan handler HTTP di atas sebuah TaskStore.
type API struct {
	store TaskStore
}

// NewAPI membuat API di atas store (interface, bukan MemoryStore).
func NewAPI(store TaskStore) *API {
	return &API{store: store}
}

// Routes mendaftarkan semua endpoint memakai pola Go 1.22 ("METHOD /path").
func (a *API) Routes() *http.ServeMux {
	panic("belum diimplementasikan")
}

// writeJSON menulis v sebagai JSON dengan status yang diminta.
// Urutan WAJIB: Header().Set(Content-Type) -> WriteHeader(status) -> Encode.
func writeJSON(w http.ResponseWriter, status int, v any) {
	panic("belum diimplementasikan")
}

// writeError menerjemahkan error aplikasi menjadi status + body errorBody.
func writeError(w http.ResponseWriter, err error) {
	panic("belum diimplementasikan")
}

// GET /tasks
func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	panic("belum diimplementasikan")
}

// POST /tasks
func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	panic("belum diimplementasikan")
}

// GET /tasks/{id}
func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	panic("belum diimplementasikan")
}

// PUT /tasks/{id}
func (a *API) updateTask(w http.ResponseWriter, r *http.Request) {
	panic("belum diimplementasikan")
}

// DELETE /tasks/{id}
func (a *API) deleteTask(w http.ResponseWriter, r *http.Request) {
	panic("belum diimplementasikan")
}
