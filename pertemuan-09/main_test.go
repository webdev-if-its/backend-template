// File ini disediakan dosen untuk mengecek progres level secara otomatis.
// JANGAN DIUBAH — perubahan pada file ini tidak akan dipakai saat penilaian
// (dosen menimpa ulang file ini sebelum menjalankan grading).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	placeholder = "(tulis di sini)"
	js          = "application/json"
)

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

func mengandungSalahSatu(teks string, kata ...string) bool {
	low := strings.ToLower(teks)
	for _, k := range kata {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// tangkapLog mengalihkan keluaran paket log ke buffer selama test berjalan.
func tangkapLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldW, oldF := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldW)
		log.SetFlags(oldF)
	})
	return &buf
}

func barisLog(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// cariBaris mencari baris log "METHOD path STATUS <durasi>" dan mengembalikan durasinya.
func cariBaris(buf *bytes.Buffer, method, path string, status int) (string, bool) {
	re := regexp.MustCompile(fmt.Sprintf(`^%s %s %d (\S+)$`, regexp.QuoteMeta(method), regexp.QuoteMeta(path), status))
	for _, l := range barisLog(buf) {
		if m := re.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			return m[1], true
		}
	}
	return "", false
}

func kirim(t *testing.T, h http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var rec *httptest.ResponseRecorder
	aman(t, func() {
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	})
	return rec
}

func harusJSON(t *testing.T, rec *httptest.ResponseRecorder, konteks string) {
	t.Helper()
	if ct := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("%s: Content-Type = %q, ingin application/json", konteks, ct)
	}
}

func bodyError(t *testing.T, rec *httptest.ResponseRecorder) (string, map[string]string, map[string]any) {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body harus JSON, dapat %q (%v)", rec.Body.String(), err)
	}
	var e struct {
		Error  string            `json:"error"`
		Detail map[string]string `json:"detail"`
	}
	json.Unmarshal(rec.Body.Bytes(), &e)
	return e.Error, e.Detail, raw
}

// bungkus memasang mw ke h; panic (mis. dari starter yang belum
// diimplementasikan) menjadi kegagalan test yang rapi.
func bungkus(t *testing.T, mw Middleware, h http.Handler) http.Handler {
	t.Helper()
	var out http.Handler
	aman(t, func() { out = mw(h) })
	return out
}

var boom = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	panic("meledak-di-handler")
})

// panicStore adalah TaskStore yang panic saat Get dipanggil.
type panicStore struct{ *MemoryStore }

func (panicStore) Get(int) (Task, error) { panic("meledak-di-store") }

func TestLevels(t *testing.T) {
	t.Run("Level 1 - Logging: satu baris per request", func(t *testing.T) {
		buf := tangkapLog(t)
		h := bungkus(t, Logging, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Uji", "ada")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("hilang"))
		}))
		rec := kirim(t, h, http.MethodGet, "/tasks/5", "", "")
		if rec.Code != 404 || rec.Body.String() != "hilang" || rec.Header().Get("X-Uji") != "ada" {
			t.Errorf("Logging harus meneruskan respons handler apa adanya: status %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("X-Uji"))
		}
		if _, ok := cariBaris(buf, "GET", "/tasks/5", 404); !ok {
			t.Errorf("log harus memuat baris persis berpola \"GET /tasks/5 404 <durasi>\" (format \"%%s %%s %%d %%v\"), dapat:\n%s", buf.String())
		}
		if n := len(barisLog(buf)); n != 1 {
			t.Errorf("harus tepat 1 baris log per request, dapat %d:\n%s", n, buf.String())
		}

		h2 := bungkus(t, Logging, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		kirim(t, h2, http.MethodPost, "/tasks", `{}`, js)
		if _, ok := cariBaris(buf, "POST", "/tasks", 201); !ok {
			t.Errorf("request kedua (POST /tasks 201) tidak tercatat dengan benar:\n%s", buf.String())
		}
		if n := len(barisLog(buf)); n != 2 {
			t.Errorf("setelah 2 request harus ada 2 baris log, dapat %d", n)
		}
	})

	t.Run("Level 2 - Logging: status default, panggilan pertama, durasi", func(t *testing.T) {
		buf := tangkapLog(t)

		// Handler yang hanya menulis body tanpa WriteHeader -> Go menjawab 200.
		kirim(t, bungkus(t, Logging, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("halo"))
		})), "GET", "/ok", "", "")
		if _, ok := cariBaris(buf, "GET", "/ok", 200); !ok {
			t.Errorf("handler tanpa WriteHeader harus tercatat 200 (status awal statusRecorder), dapat:\n%s", buf.String())
		}

		// WriteHeader dipanggil dua kali: hanya yang PERTAMA berlaku.
		buf.Reset()
		rec := kirim(t, bungkus(t, Logging, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusInternalServerError)
		})), "POST", "/dua", "", "")
		if rec.Code != 201 {
			t.Errorf("respons sebenarnya harus 201 (WriteHeader kedua diabaikan), dapat %d", rec.Code)
		}
		if _, ok := cariBaris(buf, "POST", "/dua", 201); !ok {
			t.Errorf("log harus mencatat status yang benar-benar terkirim (201), bukan panggilan WriteHeader kedua; dapat:\n%s", buf.String())
		}

		// Durasi: format yang bisa di-parse dan mencerminkan waktu handler.
		buf.Reset()
		kirim(t, bungkus(t, Logging, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(30 * time.Millisecond)
		})), "GET", "/lambat", "", "")
		durStr, ok := cariBaris(buf, "GET", "/lambat", 200)
		if !ok {
			t.Fatalf("baris log GET /lambat 200 tidak ditemukan:\n%s", buf.String())
		}
		d, err := time.ParseDuration(durStr)
		if err != nil {
			t.Fatalf("kolom durasi %q tidak bisa di-parse sebagai time.Duration (pakai %%v pada time.Since): %v", durStr, err)
		}
		if d < 25*time.Millisecond {
			t.Errorf("durasi tercatat %v, padahal handler tidur 30ms -- hitung dari SEBELUM sampai SESUDAH next.ServeHTTP", d)
		}
	})

	t.Run("Level 3 - Chain: urutan middleware", func(t *testing.T) {
		var jejak []string
		mw := func(nama string) Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					jejak = append(jejak, nama+">")
					next.ServeHTTP(w, r)
					jejak = append(jejak, "<"+nama)
				})
			}
		}
		inti := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jejak = append(jejak, "H")
			w.WriteHeader(http.StatusAccepted)
		})

		var h http.Handler
		aman(t, func() { h = Chain(inti, mw("A"), mw("B"), mw("C")) })
		rec := kirim(t, h, "GET", "/", "", "")
		if got := strings.Join(jejak, " "); got != "A> B> C> H <C <B <A" {
			t.Errorf("urutan eksekusi = %q, ingin \"A> B> C> H <C <B <A\" (middleware PERTAMA di daftar = paling luar)", got)
		}
		if rec.Code != 202 {
			t.Errorf("respons handler inti harus sampai ke klien (202), dapat %d", rec.Code)
		}

		jejak = nil
		aman(t, func() { h = Chain(inti) })
		kirim(t, h, "GET", "/", "", "")
		if got := strings.Join(jejak, " "); got != "H" {
			t.Errorf("Chain tanpa middleware harus setara handler aslinya, jejak = %q", got)
		}

		// Middleware yang memotong (tidak memanggil next) menghentikan rantai di situ.
		jejak = nil
		potong := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				jejak = append(jejak, "potong")
				w.WriteHeader(http.StatusForbidden)
			})
		}
		aman(t, func() { h = Chain(inti, mw("A"), potong, mw("B")) })
		rec = kirim(t, h, "GET", "/", "", "")
		if got := strings.Join(jejak, " "); got != "A> potong <A" || rec.Code != 403 {
			t.Errorf("middleware yang tidak memanggil next harus menghentikan rantai: jejak %q status %d", got, rec.Code)
		}
	})

	t.Run("Level 4 - Recover: panic jadi 500 JSON, server tetap hidup", func(t *testing.T) {
		tangkapLog(t)
		var h http.Handler
		aman(t, func() { h = Recover(boom) })

		for i := 1; i <= 2; i++ {
			rec := kirim(t, h, "GET", "/boom", "", "")
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("request ke-%d: handler panic -> status %d, ingin 500", i, rec.Code)
			}
			harusJSON(t, rec, "Recover")
			msg, _, _ := bodyError(t, rec)
			if msg != "terjadi kesalahan internal" {
				t.Errorf("body error = %q, ingin \"terjadi kesalahan internal\"", msg)
			}
		}

		// Nilai panic bukan string (error, struct) juga harus tertangkap.
		for _, nilai := range []any{fmt.Errorf("gagal: %w", ErrInputKosong), struct{ X int }{7}, 42} {
			nilai := nilai
			rec := kirim(t, bungkus(t, Recover, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic(nilai) })), "GET", "/", "", "")
			if rec.Code != 500 {
				t.Errorf("panic(%v) harus jadi 500, dapat %d", nilai, rec.Code)
			}
		}

		// Handler yang tidak panic tidak boleh diubah sama sekali.
		ok := bungkus(t, Recover, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Uji", "ada")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("normal"))
		}))
		rec := kirim(t, ok, "POST", "/x", "", "")
		if rec.Code != 201 || rec.Body.String() != "normal" || rec.Header().Get("X-Uji") != "ada" {
			t.Errorf("Recover tidak boleh mengubah respons handler yang tidak panic: %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("Level 5 - Recover: log lengkap, klien tidak melihat detail", func(t *testing.T) {
		buf := tangkapLog(t)
		rec := kirim(t, bungkus(t, Recover, boom), "GET", "/boom", "", "")

		lg := buf.String()
		if !strings.Contains(lg, "meledak-di-handler") {
			t.Errorf("log harus memuat nilai panic (\"meledak-di-handler\"), dapat:\n%s", lg)
		}
		if !strings.Contains(lg, "goroutine") && !strings.Contains(lg, ".go:") {
			t.Errorf("log harus memuat stack trace (debug.Stack()), supaya kalian tahu DI MANA panic terjadi; dapat:\n%s", lg)
		}
		body := strings.ToLower(rec.Body.String())
		for _, bocor := range []string{"meledak", "goroutine", ".go:", "panic"} {
			if strings.Contains(body, bocor) {
				t.Errorf("body respons membocorkan detail panic (%q): %s", bocor, rec.Body.String())
			}
		}
	})

	t.Run("Level 6 - Logging + Recover: request yang panic tetap tercatat 500", func(t *testing.T) {
		buf := tangkapLog(t)
		var h http.Handler
		aman(t, func() { h = Logging(Recover(boom)) })
		rec := kirim(t, h, "GET", "/boom", "", "")
		if rec.Code != 500 {
			t.Fatalf("status = %d, ingin 500", rec.Code)
		}
		if _, ok := cariBaris(buf, "GET", "/boom", 500); !ok {
			t.Errorf("dengan urutan Logging(Recover(h)), request yang panic harus tetap tercatat sebagai \"GET /boom 500 ...\"; dapat:\n%s", buf.String())
		}

		// Panic SETELAH sebagian respons terkirim: klien sudah menerima 200, log harus jujur mencatat 200.
		buf.Reset()
		sebagian := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("sebagian"))
			panic("meledak-di-tengah")
		})
		aman(t, func() { h = Logging(Recover(sebagian)) })
		rec = kirim(t, h, "GET", "/tengah", "", "")
		if rec.Code != 200 {
			t.Errorf("respons sudah terlanjur 200, dapat %d", rec.Code)
		}
		if _, ok := cariBaris(buf, "GET", "/tengah", 200); !ok {
			t.Errorf("status yang dicatat harus 200 (yang benar-benar terkirim ke klien), bukan 500 dari upaya WriteHeader kedua; dapat:\n%s", buf.String())
		}
	})

	t.Run("Level 7 - ValidasiTugas", func(t *testing.T) {
		valid := func(nama string, in taskInput) {
			t.Helper()
			var err error
			aman(t, func() { err = ValidasiTugas(in) })
			if err != nil {
				t.Errorf("%s: seharusnya valid, dapat error: %v", nama, err)
			}
		}
		invalid := func(nama string, in taskInput, want string) {
			t.Helper()
			var err error
			aman(t, func() { err = ValidasiTugas(in) })
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("%s: harus mengembalikan *ValidationError, dapat: %v", nama, err)
				return
			}
			if got := ve.Fields["judul"]; got != want {
				t.Errorf("%s: Fields[\"judul\"] = %q, ingin %q", nama, got, want)
			}
			if err.Error() != "validasi gagal" {
				t.Errorf("%s: Error() = %q, ingin \"validasi gagal\"", nama, err.Error())
			}
		}

		valid("judul biasa", taskInput{Judul: "Belajar middleware"})
		valid("judul 100 karakter (batas)", taskInput{Judul: strings.Repeat("a", 100)})
		valid("100 karakter dengan spasi di ujung (Trim dulu)", taskInput{Judul: "  " + strings.Repeat("a", 100) + "  "})
		valid("100 huruf Jepang = 300 byte tapi 100 karakter", taskInput{Judul: strings.Repeat("あ", 100)})
		valid("Selesai tidak divalidasi", taskInput{Judul: "x", Selesai: true})

		invalid("kosong", taskInput{Judul: ""}, "tidak boleh kosong")
		invalid("hanya spasi/tab/newline", taskInput{Judul: " \t\n "}, "tidak boleh kosong")
		invalid("101 karakter", taskInput{Judul: strings.Repeat("a", 101)}, "maksimal 100 karakter")
		invalid("101 huruf Jepang", taskInput{Judul: strings.Repeat("あ", 101)}, "maksimal 100 karakter")

		// Jebakan: mengembalikan (*ValidationError)(nil) sebagai error membuat err != nil.
		var err error
		aman(t, func() { err = ValidasiTugas(taskInput{Judul: "sah"}) })
		if err != nil {
			t.Error("input valid harus mengembalikan nil sungguhan (bukan *ValidationError bernilai nil -- jebakan 'typed nil')")
		}
	})

	t.Run("Level 8 - Validasi di API: 400 + detail", func(t *testing.T) {
		s := NewMemoryStore()
		s.Add(Task{Judul: "Awal"})
		h := NewAPI(s).Routes()

		cek := func(method, path, body string, want map[string]string) {
			t.Helper()
			rec := kirim(t, h, method, path, body, js)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s %s -> status %d, ingin 400 (body: %s)", method, path, body, rec.Code, strings.TrimSpace(rec.Body.String()))
				return
			}
			harusJSON(t, rec, method+" "+path)
			msg, detail, _ := bodyError(t, rec)
			if msg != "validasi gagal" {
				t.Errorf("%s %s: error = %q, ingin \"validasi gagal\"", method, path, msg)
			}
			for k, v := range want {
				if detail[k] != v {
					t.Errorf("%s %s: detail[%q] = %q, ingin %q", method, path, k, detail[k], v)
				}
			}
		}
		cek("POST", "/tasks", `{"judul":"   "}`, map[string]string{"judul": "tidak boleh kosong"})
		cek("POST", "/tasks", `{"selesai":true}`, map[string]string{"judul": "tidak boleh kosong"})
		cek("POST", "/tasks", `{"judul":"`+strings.Repeat("x", 101)+`"}`, map[string]string{"judul": "maksimal 100 karakter"})
		cek("PUT", "/tasks/1", `{"judul":""}`, map[string]string{"judul": "tidak boleh kosong"})
		cek("PUT", "/tasks/1", `{"judul":"`+strings.Repeat("あ", 101)+`"}`, map[string]string{"judul": "maksimal 100 karakter"})

		if got, _ := s.Get(1); got.Judul != "Awal" || len(s.List()) != 1 {
			t.Errorf("request yang ditolak validasi tidak boleh mengubah store: %+v (jumlah %d)", got, len(s.List()))
		}

		// Batas 100 karakter (termasuk huruf multibyte) harus diterima.
		if rec := kirim(t, h, "POST", "/tasks", `{"judul":"`+strings.Repeat("あ", 100)+`"}`, js); rec.Code != 201 {
			t.Errorf("judul 100 huruf Jepang harus diterima (hitung karakter, bukan byte), status %d body %s", rec.Code, rec.Body.String())
		}

		// detail hanya muncul untuk kegagalan validasi (omitempty).
		rec := kirim(t, h, "GET", "/tasks/999", "", "")
		_, _, raw := bodyError(t, rec)
		if _, ada := raw["detail"]; ada || rec.Code != 404 {
			t.Errorf("error non-validasi tidak boleh punya kunci \"detail\" (omitempty): status %d body %s", rec.Code, rec.Body.String())
		}

		// writeError memakai errors.As: error validasi yang dibungkus tetap 400 + detail.
		rec = httptest.NewRecorder()
		aman(t, func() {
			writeError(rec, fmt.Errorf("layanan: %w", &ValidationError{Fields: map[string]string{"judul": "tidak boleh kosong"}}))
		})
		if _, detail, _ := bodyError(t, rec); rec.Code != 400 || detail["judul"] != "tidak boleh kosong" {
			t.Errorf("writeError(error validasi terbungkus): status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Level 9 - Lapisan luar: RequireJSON, batas ukuran, field asing, salah tipe", func(t *testing.T) {
		// --- RequireJSON ---
		panggilan := 0
		var h http.Handler
		aman(t, func() {
			h = RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panggilan++
				w.WriteHeader(http.StatusNoContent)
			}))
		})

		for _, ct := range []string{"text/plain", "", "application/jsonx", "application/xml", "multipart/form-data; boundary=x"} {
			panggilan = 0
			rec := kirim(t, h, "POST", "/tasks", `{"judul":"x"}`, ct)
			if rec.Code != http.StatusUnsupportedMediaType {
				t.Errorf("POST dengan Content-Type %q: status %d, ingin 415", ct, rec.Code)
			}
			if panggilan != 0 {
				t.Errorf("Content-Type %q: handler tidak boleh dipanggil kalau ditolak", ct)
			}
		}
		rec := kirim(t, h, "POST", "/tasks", `{"judul":"x"}`, "text/plain")
		harusJSON(t, rec, "415")
		if msg, _, _ := bodyError(t, rec); msg != "Content-Type harus application/json" {
			t.Errorf("pesan 415 = %q, ingin \"Content-Type harus application/json\"", msg)
		}
		for _, ct := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
			if rec := kirim(t, h, "POST", "/tasks", `{}`, ct); rec.Code != http.StatusNoContent {
				t.Errorf("Content-Type %q harus diterima (parameter charset & huruf besar/kecil tidak masalah -- mime.ParseMediaType), status %d", ct, rec.Code)
			}
		}
		for _, m := range []string{"PUT", "PATCH"} {
			if rec := kirim(t, h, m, "/tasks/1", `{}`, "text/plain"); rec.Code != 415 {
				t.Errorf("%s dengan text/plain harus 415, dapat %d", m, rec.Code)
			}
		}
		for _, m := range []string{"GET", "DELETE"} {
			if rec := kirim(t, h, m, "/tasks/1", "", ""); rec.Code != http.StatusNoContent {
				t.Errorf("%s tanpa Content-Type harus tetap lolos (tidak punya body), status %d", m, rec.Code)
			}
		}

		// --- Pembacaan body di API ---
		s := NewMemoryStore()
		api := NewAPI(s).Routes()
		cek := func(nama, body string, status int, pesan string) {
			t.Helper()
			rec := kirim(t, api, "POST", "/tasks", body, js)
			if rec.Code != status {
				t.Errorf("%s: status %d, ingin %d (body: %s)", nama, rec.Code, status, strings.TrimSpace(rec.Body.String()))
				return
			}
			harusJSON(t, rec, nama)
			if got, _, _ := bodyError(t, rec); got != pesan {
				t.Errorf("%s: pesan = %q, ingin %q", nama, got, pesan)
			}
		}
		cek("body terlalu besar", `{"judul":"`+strings.Repeat("a", 5000)+`"}`, 413, "body terlalu besar")
		cek("field asing", `{"judul":"x","prioritas":1}`, 400, "body mengandung field yang tidak dikenal")
		cek("id dari klien", `{"id":5,"judul":"x"}`, 400, "body mengandung field yang tidak dikenal")
		cek("judul salah tipe", `{"judul":123}`, 400, `field "judul" harus bertipe string`)
		cek("selesai salah tipe", `{"judul":"x","selesai":"ya"}`, 400, `field "selesai" harus bertipe bool`)
		cek("bukan JSON", `{judul: x`, 400, "body bukan JSON yang valid")
		cek("terpotong", `{"judul":"x"`, 400, "body bukan JSON yang valid")

		// Body kosong sama sekali (Content-Type ada, body tidak).
		if rec := kirim(t, api, "POST", "/tasks", "", js); rec.Code != 400 {
			t.Errorf("body kosong: status %d, ingin 400", rec.Code)
		} else if got, _, _ := bodyError(t, rec); got != "body bukan JSON yang valid" {
			t.Errorf("body kosong: pesan %q", got)
		}

		// Body sah yang dipadatkan spasi mendekati batas tetap diterima.
		lolos := `{"judul":"ok"}` + strings.Repeat(" ", 900)
		if rec := kirim(t, api, "POST", "/tasks", lolos, js); rec.Code != 201 {
			t.Errorf("body sah berukuran ±900 byte harus diterima, status %d body %s", rec.Code, rec.Body.String())
		}
		// PUT memakai lapisan yang sama.
		s.Add(Task{Judul: "satu"})
		if rec := kirim(t, api, "PUT", "/tasks/1", `{"judul":"x","warna":"merah"}`, js); rec.Code != 400 {
			t.Errorf("PUT dengan field asing harus 400, dapat %d", rec.Code)
		}
		if len(s.List()) != 2 {
			t.Errorf("request yang ditolak tidak boleh menyimpan apa pun; jumlah tugas = %d, ingin 2", len(s.List()))
		}
	})

	t.Run("Level 10 - NewHandler: rantai middleware konsisten + analisis", func(t *testing.T) {
		buf := tangkapLog(t)
		var h http.Handler
		aman(t, func() { h = NewHandler(NewMemoryStore()) })

		rec := kirim(t, h, "GET", "/tasks", "", "")
		if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("GET /tasks lewat NewHandler: %d %q", rec.Code, rec.Body.String())
		}
		if _, ok := cariBaris(buf, "GET", "/tasks", 200); !ok {
			t.Errorf("Logging harus terpasang di NewHandler; log:\n%s", buf.String())
		}

		rec = kirim(t, h, "POST", "/tasks", `{"judul":"lewat rantai"}`, js)
		if rec.Code != 201 {
			t.Errorf("POST valid lewat NewHandler: status %d body %s", rec.Code, rec.Body.String())
		}

		// RequireJSON terpasang, dan penolakannya tetap tercatat oleh Logging (Logging paling luar).
		rec = kirim(t, h, "POST", "/tasks", `{"judul":"x"}`, "text/plain")
		if rec.Code != 415 {
			t.Errorf("POST text/plain lewat NewHandler: status %d, ingin 415 (RequireJSON belum terpasang?)", rec.Code)
		}
		if _, ok := cariBaris(buf, "POST", "/tasks", 415); !ok {
			t.Errorf("penolakan 415 harus tercatat di log (Logging harus paling luar); log:\n%s", buf.String())
		}

		// Konsisten: rantai membungkus SELURUH mux, termasuk rute yang tidak ada.
		rec = kirim(t, h, "GET", "/nope", "", "")
		if rec.Code != 404 {
			t.Errorf("GET /nope status %d, ingin 404", rec.Code)
		}
		if _, ok := cariBaris(buf, "GET", "/nope", 404); !ok {
			t.Errorf("rute tak dikenal juga harus tercatat -- pasang rantai ke SELURUH mux, bukan per-route; log:\n%s", buf.String())
		}

		// Recover terpasang di dalam Logging: panic di lapisan store -> 500 JSON + tercatat 500.
		buf.Reset()
		var hp http.Handler
		aman(t, func() { hp = NewHandler(panicStore{NewMemoryStore()}) })
		rec = kirim(t, hp, "GET", "/tasks/1", "", "")
		if rec.Code != 500 {
			t.Fatalf("panic di store lewat NewHandler: status %d, ingin 500 (Recover belum terpasang?)", rec.Code)
		}
		harusJSON(t, rec, "500")
		if msg, _, _ := bodyError(t, rec); msg != "terjadi kesalahan internal" {
			t.Errorf("pesan 500 = %q", msg)
		}
		if _, ok := cariBaris(buf, "GET", "/tasks/1", 500); !ok {
			t.Errorf("request yang panic harus tercatat GET /tasks/1 500 (Logging di luar Recover); log:\n%s", buf.String())
		}
		// Server tetap melayani request berikutnya setelah panic.
		if rec := kirim(t, hp, "GET", "/tasks", "", ""); rec.Code != 200 {
			t.Errorf("setelah satu request panic, request lain harus tetap dilayani; status %d", rec.Code)
		}

		s := section(readFile("../README.md"), "Analisis Pertemuan 9")
		if !filled(s, 200) {
			t.Fatalf("section '## Analisis Pertemuan 9' di README.md (root repo) tidak ada atau belum diisi memadai (minimal 200 karakter, saat ini %d) -- tambahkan heading persis itu kalau belum ada", len(strings.TrimSpace(s)))
		}
		if !mengandungSalahSatu(s, "recover") {
			t.Error("analisis harus menyebut recover")
		}
		if !mengandungSalahSatu(s, "goroutine") {
			t.Error("analisis harus membahas goroutine (kenapa Recover tidak menangkap panic di goroutine yang dibuat handler)")
		}
		if !mengandungSalahSatu(s, "urutan", "luar", "dalam", "order") {
			t.Error("analisis harus membahas urutan/posisi middleware (mana yang di luar, mana yang di dalam)")
		}
	})
}
