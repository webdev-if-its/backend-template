// File ini disediakan dosen untuk mengecek progres level secara otomatis.
// JANGAN DIUBAH — perubahan pada file ini tidak akan dipakai saat penilaian
// (dosen menimpa ulang file ini sebelum menjalankan grading).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

const placeholder = "(tulis di sini)"

var headingRe = regexp.MustCompile(`(?im)^##\s+(.+?)\s*$`)

func aman(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("fungsi panic: %v (kemungkinan belum diimplementasikan)", r)
		}
	}()
	fn()
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func section(readme, heading string) string {
	locs := headingRe.FindAllStringSubmatchIndex(readme, -1)
	headings := headingRe.FindAllStringSubmatch(readme, -1)
	for i, h := range headings {
		if strings.EqualFold(strings.TrimSpace(h[1]), heading) {
			start := locs[i][1]
			end := len(readme)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			return strings.TrimSpace(readme[start:end])
		}
	}
	return ""
}

func filled(s string, minLen int) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, placeholder) {
		return false
	}
	return len(s) >= minLen
}

// errStore selalu gagal dengan error yang berisi "rahasia" -- dipakai untuk
// memastikan detail error internal tidak bocor ke klien.
type errStore struct{}

var errRahasia = errors.New("koneksi db meledak: password=rahasia123 host=10.0.0.5")

func (errStore) Add(Task) (Task, error)    { return Task{}, errRahasia }
func (errStore) Get(int) (Task, error)     { return Task{}, errRahasia }
func (errStore) List() []Task              { return nil }
func (errStore) Update(Task) (Task, error) { return Task{}, errRahasia }
func (errStore) Delete(int) error          { return errRahasia }

// kirim memanggil handler API lewat httptest (tanpa membuka port sungguhan).
func kirim(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rec *httptest.ResponseRecorder
	aman(t, func() {
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		}
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	})
	return rec
}

func routes(t *testing.T, store TaskStore) http.Handler {
	t.Helper()
	var h http.Handler
	aman(t, func() { h = NewAPI(store).Routes() })
	return h
}

func decodeTask(t *testing.T, rec *httptest.ResponseRecorder) Task {
	t.Helper()
	var tk Task
	if err := json.Unmarshal(rec.Body.Bytes(), &tk); err != nil {
		t.Fatalf("body bukan JSON Task yang valid: %v\nbody: %s", err, rec.Body.String())
	}
	return tk
}

func isiTiga(t *testing.T) (*MemoryStore, http.Handler) {
	t.Helper()
	s := NewMemoryStore()
	s.Add(Task{Judul: "Belajar Go"})
	s.Add(Task{Judul: "Push ke GitHub"})
	s.Add(Task{Judul: "Baca net/http"})
	return s, routes(t, s)
}

func pesanError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body error harus JSON {\"error\":\"...\"}, dapat: %q (%v)", rec.Body.String(), err)
	}
	return e.Error
}

func harusJSON(t *testing.T, rec *httptest.ResponseRecorder, konteks string) {
	t.Helper()
	// Result() memotret header saat WriteHeader dipanggil, seperti server sungguhan:
	// Header().Set yang terlambat (setelah WriteHeader) tidak ikut terkirim.
	if ct := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("%s: Content-Type = %q, ingin application/json (Header().Set harus SEBELUM WriteHeader)", konteks, ct)
	}
}

func TestLevels(t *testing.T) {
	t.Run("Level 1 - GET /tasks (JSON)", func(t *testing.T) {
		// writeJSON langsung: urutan Header -> WriteHeader -> Encode.
		rec := httptest.NewRecorder()
		aman(t, func() { writeJSON(rec, http.StatusTeapot, map[string]int{"a": 1}) })
		if rec.Code != http.StatusTeapot {
			t.Errorf("writeJSON status = %d, ingin %d", rec.Code, http.StatusTeapot)
		}
		harusJSON(t, rec, "writeJSON")
		if strings.TrimSpace(rec.Body.String()) != `{"a":1}` {
			t.Errorf("writeJSON body = %q, ingin {\"a\":1}", rec.Body.String())
		}

		_, h := isiTiga(t)
		rec = kirim(t, h, http.MethodGet, "/tasks", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /tasks status = %d, ingin 200", rec.Code)
		}
		harusJSON(t, rec, "GET /tasks")
		var daftar []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &daftar); err != nil {
			t.Fatalf("body GET /tasks bukan array JSON: %v\nbody: %s", err, rec.Body.String())
		}
		if len(daftar) != 3 || daftar[0]["judul"] != "Belajar Go" || daftar[2]["judul"] != "Baca net/http" {
			t.Errorf("GET /tasks harus memuat 3 tugas sesuai urutan, dapat: %s", rec.Body.String())
		}
		for _, kunci := range []string{"id", "judul", "selesai"} {
			if _, ok := daftar[0][kunci]; !ok {
				t.Errorf("tugas di JSON harus punya kunci %q (cek tag json), dapat: %v", kunci, daftar[0])
			}
		}

		// Daftar kosong harus [] bukan null.
		kosong := kirim(t, routes(t, NewMemoryStore()), http.MethodGet, "/tasks", "")
		if strings.TrimSpace(kosong.Body.String()) != "[]" {
			t.Errorf("GET /tasks pada store kosong harus '[]', bukan %q", strings.TrimSpace(kosong.Body.String()))
		}
	})

	t.Run("Level 2 - POST /tasks", func(t *testing.T) {
		s := NewMemoryStore()
		h := routes(t, s)
		rec := kirim(t, h, http.MethodPost, "/tasks", `{"judul":"Belajar REST"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST /tasks status = %d, ingin 201 Created (body: %s)", rec.Code, rec.Body.String())
		}
		harusJSON(t, rec, "POST /tasks")
		tk := decodeTask(t, rec)
		if tk.ID == 0 || tk.Judul != "Belajar REST" || tk.Selesai {
			t.Errorf("respons POST = %+v, ingin ID terisi, Judul \"Belajar REST\", Selesai false", tk)
		}
		if got := s.List(); len(got) != 1 || got[0].Judul != "Belajar REST" {
			t.Errorf("tugas harus benar-benar tersimpan di store, isi store: %+v", got)
		}

		rec = kirim(t, h, http.MethodPost, "/tasks", `{"judul":"Sudah beres","selesai":true}`)
		if tk2 := decodeTask(t, rec); rec.Code != 201 || !tk2.Selesai || tk2.ID == tk.ID {
			t.Errorf("POST dengan selesai=true: status %d, %+v (ID harus beda dari tugas pertama)", rec.Code, tk2)
		}
	})

	t.Run("Level 3 - GET /tasks/{id}", func(t *testing.T) {
		_, h := isiTiga(t)
		rec := kirim(t, h, http.MethodGet, "/tasks/2", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /tasks/2 status = %d, ingin 200 (go.mod harus 'go 1.22' atau lebih agar pola \"GET /tasks/{id}\" berfungsi)", rec.Code)
		}
		harusJSON(t, rec, "GET /tasks/2")
		if tk := decodeTask(t, rec); tk.ID != 2 || tk.Judul != "Push ke GitHub" {
			t.Errorf("GET /tasks/2 = %+v, ingin ID 2 \"Push ke GitHub\"", tk)
		}
		rec = kirim(t, h, http.MethodGet, "/tasks/3", "")
		if tk := decodeTask(t, rec); tk.ID != 3 {
			t.Errorf("GET /tasks/3 mengembalikan ID %d -- id harus diambil dari path (r.PathValue)", tk.ID)
		}
	})

	t.Run("Level 4 - PUT /tasks/{id} (penggantian penuh)", func(t *testing.T) {
		s, h := isiTiga(t)
		s.Update(Task{ID: 2, Judul: "Push ke GitHub", Selesai: true})

		rec := kirim(t, h, http.MethodPut, "/tasks/2", `{"judul":"Push & buat PR","selesai":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT /tasks/2 status = %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
		}
		harusJSON(t, rec, "PUT /tasks/2")
		if tk := decodeTask(t, rec); tk.ID != 2 || tk.Judul != "Push & buat PR" || !tk.Selesai {
			t.Errorf("respons PUT = %+v", tk)
		}
		if got, _ := s.Get(2); got.Judul != "Push & buat PR" || !got.Selesai {
			t.Errorf("perubahan harus tersimpan di store, isi: %+v", got)
		}

		// PUT = penggantian PENUH: field yang tidak dikirim kembali ke nilai nol.
		rec = kirim(t, h, http.MethodPut, "/tasks/2", `{"judul":"Hanya judul"}`)
		if tk := decodeTask(t, rec); tk.Selesai {
			t.Errorf("PUT tanpa field selesai harus mengembalikannya ke false (penggantian penuh), dapat %+v", tk)
		}
		// Tugas lain tidak tersentuh.
		if got, _ := s.Get(1); got.Judul != "Belajar Go" {
			t.Errorf("PUT /tasks/2 tidak boleh mengubah tugas lain: %+v", got)
		}
	})

	t.Run("Level 5 - DELETE /tasks/{id}", func(t *testing.T) {
		s, h := isiTiga(t)
		rec := kirim(t, h, http.MethodDelete, "/tasks/2", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("DELETE /tasks/2 status = %d, ingin 204 No Content", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("204 tidak boleh punya body, dapat %q", rec.Body.String())
		}
		if _, err := s.Get(2); !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("tugas harus benar-benar terhapus dari store, Get -> err %v", err)
		}
		if len(s.List()) != 2 {
			t.Errorf("tersisa %d tugas, ingin 2", len(s.List()))
		}
		// Menghapus lagi: 404 (status saja di level ini).
		if rec := kirim(t, h, http.MethodDelete, "/tasks/2", ""); rec.Code != http.StatusNotFound {
			t.Errorf("DELETE id yang sudah tidak ada status = %d, ingin 404", rec.Code)
		}
	})

	t.Run("Level 6 - Kode status untuk kasus gagal", func(t *testing.T) {
		_, h := isiTiga(t)
		cek := func(nama, method, path, body string, want int) {
			t.Helper()
			if rec := kirim(t, h, method, path, body); rec.Code != want {
				t.Errorf("%s: %s %s -> status %d, ingin %d (body: %s)", nama, method, path, rec.Code, want, strings.TrimSpace(rec.Body.String()))
			}
		}
		cek("id tidak ada (GET)", "GET", "/tasks/999", "", 404)
		cek("id tidak ada (PUT)", "PUT", "/tasks/999", `{"judul":"x"}`, 404)
		cek("id tidak ada (DELETE)", "DELETE", "/tasks/999", "", 404)
		cek("id bukan angka (GET)", "GET", "/tasks/abc", "", 400)
		cek("id bukan angka (PUT)", "PUT", "/tasks/abc", `{"judul":"x"}`, 400)
		cek("id bukan angka (DELETE)", "DELETE", "/tasks/abc", "", 400)
		cek("body bukan JSON (POST)", "POST", "/tasks", `{judul: bukan json`, 400)
		cek("body kosong (POST)", "POST", "/tasks", "", 400)
		cek("tipe salah (POST)", "POST", "/tasks", `{"judul":123}`, 400)
		cek("judul kosong (POST)", "POST", "/tasks", `{"judul":"   "}`, 400)
		cek("judul kosong (PUT)", "PUT", "/tasks/1", `{"judul":""}`, 400)
		cek("body bukan JSON (PUT)", "PUT", "/tasks/1", `[[[`, 400)

		// Urutan pengecekan: body rusak pada id yang tidak ada -> tetap kesalahan klien.
		if rec := kirim(t, h, "PUT", "/tasks/999", `[[[`); rec.Code != 400 && rec.Code != 404 {
			t.Errorf("PUT id tak ada dengan body rusak: status %d, ingin 400 atau 404", rec.Code)
		}
		// Kegagalan tidak boleh mengubah data.
		if rec := kirim(t, h, "GET", "/tasks", ""); strings.Count(rec.Body.String(), `"id"`) != 3 {
			t.Errorf("request gagal tidak boleh mengubah isi store: %s", rec.Body.String())
		}
	})

	t.Run("Level 7 - Body error terstruktur {\"error\":\"...\"}", func(t *testing.T) {
		_, h := isiTiga(t)
		cek := func(method, path, body, want string) {
			t.Helper()
			rec := kirim(t, h, method, path, body)
			harusJSON(t, rec, method+" "+path)
			if got := pesanError(t, rec); got != want {
				t.Errorf("%s %s: pesan error = %q, ingin %q", method, path, got, want)
			}
		}
		cek("GET", "/tasks/999", "", "tugas tidak ditemukan")
		cek("DELETE", "/tasks/999", "", "tugas tidak ditemukan")
		cek("PUT", "/tasks/999", `{"judul":"x"}`, "tugas tidak ditemukan")
		cek("GET", "/tasks/abc", "", "id harus berupa angka")
		cek("POST", "/tasks", `{judul: bukan json`, "body bukan JSON yang valid")
		cek("POST", "/tasks", "", "body bukan JSON yang valid")
		cek("POST", "/tasks", `{"judul":123}`, "body bukan JSON yang valid")
		cek("POST", "/tasks", `{"judul":"  "}`, "input tidak boleh kosong")
		cek("PUT", "/tasks/1", `{"judul":""}`, "input tidak boleh kosong")

		// writeError langsung: harus memakai errors.Is, bukan ==, jadi error yang dibungkus tetap terbaca.
		rec := httptest.NewRecorder()
		aman(t, func() { writeError(rec, fmt.Errorf("get tugas 7: %w", ErrTugasTidakDitemukan)) })
		if rec.Code != 404 || pesanError(t, rec) != "tugas tidak ditemukan" {
			t.Errorf("writeError dengan error terbungkus %%w: status %d body %q, ingin 404 \"tugas tidak ditemukan\" (pakai errors.Is)", rec.Code, rec.Body.String())
		}
		rec = httptest.NewRecorder()
		aman(t, func() { writeError(rec, fmt.Errorf("validasi: %w", ErrInputKosong)) })
		if rec.Code != 400 {
			t.Errorf("writeError(ErrInputKosong terbungkus) status = %d, ingin 400", rec.Code)
		}
	})

	t.Run("Level 8 - Error internal: 500 generik, tanpa membocorkan detail", func(t *testing.T) {
		h := routes(t, errStore{})
		for _, c := range []struct{ method, path, body string }{
			{"POST", "/tasks", `{"judul":"x"}`},
			{"GET", "/tasks/1", ""},
			{"PUT", "/tasks/1", `{"judul":"x"}`},
			{"DELETE", "/tasks/1", ""},
		} {
			rec := kirim(t, h, c.method, c.path, c.body)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("%s %s: store gagal dengan error tak dikenal -> status %d, ingin 500", c.method, c.path, rec.Code)
				continue
			}
			harusJSON(t, rec, c.method+" "+c.path)
			if got := pesanError(t, rec); got != "terjadi kesalahan internal" {
				t.Errorf("%s %s: pesan = %q, ingin \"terjadi kesalahan internal\"", c.method, c.path, got)
			}
			low := strings.ToLower(rec.Body.String())
			for _, bocor := range []string{"rahasia", "password", "10.0.0.5", "meledak", "koneksi"} {
				if strings.Contains(low, bocor) {
					t.Errorf("%s %s: body membocorkan detail internal (%q): %s -- catat detailnya ke log, jangan kirim ke klien", c.method, c.path, bocor, rec.Body.String())
				}
			}
		}
	})

	t.Run("Level 9 - Kontrak: ID bukan urusan klien & bentuk JSON stabil", func(t *testing.T) {
		s := NewMemoryStore()
		h := routes(t, s)

		// Klien mencoba menentukan ID sendiri: harus diabaikan.
		rec := kirim(t, h, http.MethodPost, "/tasks", `{"id":999,"judul":"Curang"}`)
		tk := decodeTask(t, rec)
		if tk.ID == 999 {
			t.Error("POST: ID dari body harus diabaikan; ID hanya boleh dibuat oleh store")
		}
		if _, err := s.Get(999); err == nil {
			t.Error("tugas tidak boleh tersimpan dengan ID pilihan klien (999)")
		}

		// PUT: ID di path yang menang, bukan ID di body.
		s.Add(Task{Judul: "Kedua"})
		rec = kirim(t, h, http.MethodPut, fmt.Sprintf("/tasks/%d", tk.ID), `{"id":2,"judul":"Diubah"}`)
		if got := decodeTask(t, rec); got.ID != tk.ID || got.Judul != "Diubah" {
			t.Errorf("PUT dengan id di body berbeda: respons %+v, ingin ID %d (path menang) judul \"Diubah\"", got, tk.ID)
		}
		if dua, _ := s.Get(2); dua.Judul != "Kedua" {
			t.Errorf("tugas ID 2 tidak boleh tertimpa lewat PUT ke ID lain: %+v", dua)
		}

		// Bentuk JSON persis 3 kunci, dengan tipe yang benar.
		rec = kirim(t, h, http.MethodGet, fmt.Sprintf("/tasks/%d", tk.ID), "")
		var raw map[string]any
		json.Unmarshal(rec.Body.Bytes(), &raw)
		if len(raw) != 3 {
			t.Errorf("JSON Task harus punya persis kunci id, judul, selesai; dapat %v", raw)
		}
		if _, ok := raw["id"].(float64); !ok {
			t.Errorf("id harus berupa angka di JSON, dapat %T", raw["id"])
		}
		if _, ok := raw["selesai"].(bool); !ok {
			t.Errorf("selesai harus boolean di JSON, dapat %T", raw["selesai"])
		}

		// Metode yang tidak didukung / rute tak dikenal ditolak (bukan sukses).
		for _, c := range []struct{ method, path string }{{"PATCH", "/tasks/1"}, {"POST", "/tasks/1"}, {"GET", "/nope"}} {
			if rec := kirim(t, h, c.method, c.path, ""); rec.Code < 400 || rec.Code >= 500 {
				t.Errorf("%s %s harus ditolak dengan 4xx (405/404), dapat %d", c.method, c.path, rec.Code)
			}
		}
	})

	t.Run("Level 10 - Dokumentasi endpoint di README", func(t *testing.T) {
		s := section(readFile("../README.md"), "Dokumentasi API Task Manager")
		if !filled(s, 500) {
			t.Fatalf("section '## Dokumentasi API Task Manager' di README.md (root repo) tidak ada atau belum diisi memadai (minimal 500 karakter, saat ini %d) -- tambahkan heading persis itu kalau belum ada", len(strings.TrimSpace(s)))
		}
		endpoints := []struct{ method, path string }{
			{"GET", "/tasks"}, {"POST", "/tasks"}, {"GET", "/tasks/{id}"}, {"PUT", "/tasks/{id}"}, {"DELETE", "/tasks/{id}"},
		}
		for _, e := range endpoints {
			re := regexp.MustCompile(`(?i)` + e.method + `\W+` + regexp.QuoteMeta(e.path) + `(\W|$)`)
			if !re.MatchString(s) {
				t.Errorf("dokumentasi belum memuat endpoint %s %s", e.method, e.path)
			}
		}
		for _, kode := range []string{"200", "201", "204", "400", "404", "500"} {
			if !strings.Contains(s, kode) {
				t.Errorf("dokumentasi belum menyebut kode status %s", kode)
			}
		}
		if !strings.Contains(s, `"judul"`) {
			t.Error(`dokumentasi harus memuat minimal satu contoh body JSON (mis. {"judul": "..."})`)
		}
		if !strings.Contains(s, `"error"`) {
			t.Error(`dokumentasi harus memuat contoh respons error {"error": "..."}`)
		}
		low := strings.ToLower(s)
		if !strings.Contains(low, "log") && !strings.Contains(low, "internal") {
			t.Error("dokumentasi harus menjelaskan kenapa error 500 tidak menampilkan detail asli ke klien (dicatat ke log / pesan internal)")
		}
	})
}
