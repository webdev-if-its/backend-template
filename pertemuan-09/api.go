package main

// api.go -- API dari Pertemuan 6 yang sudah berjalan. Tugas ini MENGEMBANGKAN
// file ini di dua tempat yang ditandai TODO (Level 8 dan Level 9); sisanya
// sudah bekerja dan tidak perlu diubah.

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// maxBodyBytes adalah batas ukuran body request. (Nilai kecil sengaja dipakai
// agar mudah diuji; di produksi biasanya 1 MB.)
const maxBodyBytes = 1024

type taskInput struct {
	Judul   string `json:"judul"`
	Selesai bool   `json:"selesai"`
}

// errorBody: {"error":"pesan"} dan, khusus kegagalan validasi, ditambah
// "detail": {"<field>": "<alasan>"} (omitempty -> tidak muncul kalau kosong).
type errorBody struct {
	Error  string            `json:"error"`
	Detail map[string]string `json:"detail,omitempty"`
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

// TODO(Level 8): tambahkan case untuk kegagalan validasi (*ValidationError,
// pakai errors.As supaya error yang dibungkus tetap terbaca) -> status 400,
// body {"error":"validasi gagal","detail":{...}}.
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

// TODO(Level 9): perketat pembacaan body -- batasi ukuran (maxBodyBytes,
// http.MaxBytesReader) -> 413; tolak field tak dikenal
// (Decoder.DisallowUnknownFields) -> 400; bedakan salah tipe field
// (*json.UnmarshalTypeError) -> 400. Pesan persisnya ada di SOAL.md.
func decodeInput(w http.ResponseWriter, r *http.Request) (taskInput, bool) {
	var in taskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "body bukan JSON yang valid"})
		return in, false
	}
	return in, true
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.List())
}

// TODO(Level 8): panggil ValidasiTugas(in) sebelum menyimpan; kalau error,
// teruskan ke writeError.
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

// TODO(Level 8): panggil ValidasiTugas(in) sebelum memperbarui.
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
