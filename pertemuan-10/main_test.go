// File ini disediakan dosen untuk mengecek progres level secara otomatis.
// JANGAN DIUBAH — perubahan pada file ini tidak akan dipakai saat penilaian
// (dosen menimpa ulang file ini sebelum menjalankan grading).
//
// Test ini BUTUH PostgreSQL yang menyala (default: localhost:5433, lihat
// dsnUji; bisa diganti lewat variabel lingkungan DATABASE_URL). Tiap level
// bekerja di SCHEMA sementara miliknya sendiri (search_path), jadi banyak
// test/mahasiswa bisa berbagi satu server tanpa saling mengganggu, dan tabel
// tidak pernah menyentuh schema public. Karena itu kode kalian TIDAK boleh
// menulis nama tabel dengan awalan schema ("public.tasks").
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	placeholder = "(tulis di sini)"
	dsnUji      = "postgres://taskmanager:rahasia@localhost:5433/taskmanager?sslmode=disable"
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

// dalam menjalankan fn di goroutine tersendiri dengan batas waktu d. Di dalam
// fn pakai t.Errorf, JANGAN t.Fatalf.
func dalam(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	selesai := make(chan any, 1)
	go func() {
		defer func() { selesai <- recover() }()
		fn()
	}()
	select {
	case r := <-selesai:
		if r != nil {
			t.Fatalf("fungsi panic: %v (kemungkinan belum diimplementasikan)", r)
		}
	case <-time.After(d):
		t.Fatalf("melebihi batas waktu %v -- macet/menunggu tanpa akhir? (koneksi tidak dikembalikan ke pool? tidak ada timeout?)", d)
	}
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

func baseDSN() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return dsnUji
}

// env adalah lingkungan uji satu level: schema sementara + koneksi ke dalamnya.
type env struct {
	db     *sql.DB
	dsn    string // DSN dengan search_path ke schema sementara
	schema string
	store  *PostgresStore
}

const ddlTabel = `
CREATE TABLE tasks (
	id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	judul   TEXT NOT NULL,
	selesai BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE task_logs (
	id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	task_id INT NOT NULL,
	aksi    TEXT NOT NULL,
	waktu   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

func bukaDBUji(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { db.Close() })
	return db
}

// siapkan membuat schema sementara. Kalau dgTabel, tabel tasks & task_logs
// dibuat oleh test (bukan oleh Migrate kalian) supaya level lain tidak
// bergantung pada Level 2.
func siapkan(t *testing.T, dgTabel bool) *env {
	t.Helper()
	dsn := baseDSN()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("DSN tidak valid: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("PostgreSQL tidak bisa dihubungi (%v).\nJalankan database dulu -- mis. `docker compose up -d --wait` di folder pertemuan-07 (file docker-compose.yml ditulis di Level 10),\natau arahkan variabel lingkungan DATABASE_URL ke server PostgreSQL yang aktif.\nDSN yang dicoba: %s", err, sanitasiDSN(dsn))
	}

	b := make([]byte, 6)
	rand.Read(b)
	schema := "t_" + hex.EncodeToString(b)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("gagal membuat schema uji: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		admin.ExecContext(c, "DROP SCHEMA "+schema+" CASCADE")
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN tidak bisa di-parse: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db := bukaDBUji(t, u.String())
	if dgTabel {
		if _, err := db.Exec(ddlTabel); err != nil {
			t.Fatalf("gagal membuat tabel uji: %v", err)
		}
	}
	return &env{db: db, dsn: u.String(), schema: schema, store: NewPostgresStore(db)}
}

func sanitasiDSN(dsn string) string {
	if u, err := url.Parse(dsn); err == nil {
		if u.User != nil {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
		return u.String()
	}
	return "(tidak valid)"
}

// seed menyisipkan satu tugas langsung lewat SQL (tanpa memakai Add milik
// kalian) dan mengembalikan ID-nya, supaya level lain tidak bergantung pada Level 3.
func seed(t *testing.T, db *sql.DB, judul string) int {
	t.Helper()
	var id int
	if err := db.QueryRow(`INSERT INTO tasks (judul) VALUES ($1) RETURNING id`, judul).Scan(&id); err != nil {
		t.Fatalf("seed gagal: %v", err)
	}
	return id
}

func hitung(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("query hitung gagal (%s): %v", query, err)
	}
	return n
}

type kolom struct{ tipe, nullable, def, identity string }

func kolomTabel(t *testing.T, db *sql.DB, tabel string) map[string]kolom {
	t.Helper()
	rows, err := db.Query(`SELECT column_name, data_type, is_nullable, COALESCE(column_default, ''), is_identity
		FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1`, tabel)
	if err != nil {
		t.Fatalf("gagal membaca information_schema: %v", err)
	}
	defer rows.Close()
	out := map[string]kolom{}
	for rows.Next() {
		var nama string
		var k kolom
		if err := rows.Scan(&nama, &k.tipe, &k.nullable, &k.def, &k.identity); err != nil {
			t.Fatal(err)
		}
		out[nama] = k
	}
	return out
}

// errListStore adalah TaskStore yang List()-nya selalu gagal dengan error berisi detail rahasia.
type errListStore struct{ *MemoryStore }

func (errListStore) List() ([]Task, error) {
	return nil, errors.New("koneksi database putus: password=rahasia123")
}

func TestLevels(t *testing.T) {
	t.Run("Level 1 - Connect (Open + Ping + pool)", func(t *testing.T) {
		dsn := baseDSN()
		var db *sql.DB
		var err error
		dalam(t, 15*time.Second, func() { db, err = Connect(dsn) })
		if err != nil {
			t.Fatalf("Connect ke DSN yang benar gagal: %v\n(pastikan PostgreSQL menyala: docker compose up -d --wait, atau atur DATABASE_URL)", err)
		}
		defer db.Close()
		if db == nil {
			t.Fatal("Connect mengembalikan db nil tanpa error")
		}
		if got := db.Stats().MaxOpenConnections; got != 10 {
			t.Errorf("MaxOpenConnections = %d, ingin 10 (db.SetMaxOpenConns(10)); jangan biarkan pool tanpa batas", got)
		}
		if err := db.Ping(); err != nil {
			t.Errorf("db yang dikembalikan Connect harus bisa di-Ping: %v", err)
		}

		u, perr := url.Parse(dsn)
		if perr != nil {
			t.Fatalf("DSN uji tidak valid: %v", perr)
		}

		// sql.Open TIDAK pernah memeriksa koneksi -- kegagalan baru ketahuan lewat Ping.
		bad := *u
		bad.Path = "/db_tidak_ada_xyz"
		var db2 *sql.DB
		dalam(t, 15*time.Second, func() { db2, err = Connect(bad.String()) })
		if err == nil {
			if db2 != nil {
				db2.Close()
			}
			t.Error("Connect ke database yang tidak ada harus mengembalikan error (ingat: sql.Open sendiri tidak error -- kegagalan baru muncul saat Ping)")
		}

		mati := *u
		mati.Host = u.Hostname() + ":1"
		var db3 *sql.DB
		dalam(t, 15*time.Second, func() { db3, err = Connect(mati.String()) })
		if err == nil {
			if db3 != nil {
				db3.Close()
			}
			t.Error("Connect ke port yang tidak ada yang mendengarkan harus mengembalikan error")
		}

		var db4 *sql.DB
		dalam(t, 5*time.Second, func() { db4, err = Connect("ini bukan dsn sama sekali ::::") })
		if err == nil {
			if db4 != nil {
				db4.Close()
			}
			t.Error("Connect dengan DSN sampah harus mengembalikan error")
		}
	})

	t.Run("Level 2 - Migrate: skema tasks & task_logs, idempotent", func(t *testing.T) {
		e := siapkan(t, false)
		for i := 1; i <= 2; i++ {
			var err error
			aman(t, func() { err = e.store.Migrate() })
			if err != nil {
				t.Fatalf("Migrate ke-%d gagal: %v (harus aman dipanggil berkali-kali -> CREATE TABLE IF NOT EXISTS)", i, err)
			}
			if i == 1 {
				if _, err := e.db.Exec(`INSERT INTO tasks (judul) VALUES ('harus tetap ada')`); err != nil {
					t.Fatalf("tabel tasks belum bisa dipakai setelah Migrate: %v", err)
				}
			}
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks`); n != 1 {
			t.Errorf("data harus selamat dari Migrate kedua (jangan DROP TABLE), jumlah baris = %d", n)
		}

		tasks := kolomTabel(t, e.db, "tasks")
		cek := func(tabel string, kol map[string]kolom, nama, tipe, nullable string) kolom {
			t.Helper()
			k, ok := kol[nama]
			if !ok {
				t.Errorf("%s: kolom %q tidak ada", tabel, nama)
				return k
			}
			if k.tipe != tipe {
				t.Errorf("%s.%s bertipe %q, ingin %q", tabel, nama, k.tipe, tipe)
			}
			if k.nullable != nullable {
				t.Errorf("%s.%s is_nullable = %q, ingin %q", tabel, nama, k.nullable, nullable)
			}
			return k
		}
		if k := cek("tasks", tasks, "id", "integer", "NO"); k.identity != "YES" {
			t.Errorf("tasks.id harus GENERATED ALWAYS AS IDENTITY (is_identity=%q)", k.identity)
		}
		cek("tasks", tasks, "judul", "text", "NO")
		if k := cek("tasks", tasks, "selesai", "boolean", "NO"); !strings.Contains(strings.ToLower(k.def), "false") {
			t.Errorf("tasks.selesai harus DEFAULT FALSE, default = %q", k.def)
		}
		if len(tasks) != 3 {
			t.Errorf("tasks harus punya persis 3 kolom (id, judul, selesai), dapat %d", len(tasks))
		}

		logs := kolomTabel(t, e.db, "task_logs")
		if k := cek("task_logs", logs, "id", "integer", "NO"); k.identity != "YES" {
			t.Errorf("task_logs.id harus IDENTITY")
		}
		cek("task_logs", logs, "task_id", "integer", "NO")
		cek("task_logs", logs, "aksi", "text", "NO")
		if k := cek("task_logs", logs, "waktu", "timestamp with time zone", "NO"); !strings.Contains(strings.ToLower(k.def), "now") && !strings.Contains(strings.ToLower(k.def), "current_timestamp") {
			t.Errorf("task_logs.waktu harus DEFAULT NOW(), default = %q", k.def)
		}
		if len(logs) != 4 {
			t.Errorf("task_logs harus punya persis 4 kolom (id, task_id, aksi, waktu), dapat %d", len(logs))
		}
	})

	t.Run("Level 3 - Add (INSERT ... RETURNING, aman dari SQL injection)", func(t *testing.T) {
		e := siapkan(t, true)
		var a, b Task
		var err error
		aman(t, func() { a, err = e.store.Add(Task{Judul: "  Belajar SQL  "}) })
		if err != nil {
			t.Fatalf("Add gagal: %v", err)
		}
		if a.ID <= 0 || a.Judul != "Belajar SQL" || a.Selesai {
			t.Errorf("Add mengembalikan ID=%d Judul=%q Selesai=%v, ingin ID>0 Judul=\"Belajar SQL\" (spasi dibuang) Selesai=false", a.ID, a.Judul, a.Selesai)
		}
		aman(t, func() { b, err = e.store.Add(Task{Judul: "Sudah beres", Selesai: true}) })
		if err != nil || b.ID <= a.ID || !b.Selesai {
			t.Errorf("Add kedua: ID=%d (harus > %d), Selesai=%v (harus true), err=%v", b.ID, a.ID, b.Selesai, err)
		}
		var judul string
		var selesai bool
		if err := e.db.QueryRow(`SELECT judul, selesai FROM tasks WHERE id = $1`, b.ID).Scan(&judul, &selesai); err != nil || judul != "Sudah beres" || !selesai {
			t.Errorf("baris di database untuk ID %d: judul=%q selesai=%v err=%v", b.ID, judul, selesai, err)
		}

		for _, kosong := range []string{"", "   ", "\t\n"} {
			aman(t, func() { _, err = e.store.Add(Task{Judul: kosong}) })
			if !errors.Is(err, ErrInputKosong) {
				t.Errorf("Add judul %q harus ErrInputKosong, dapat: %v", kosong, err)
			}
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks`); n != 2 {
			t.Errorf("judul kosong tidak boleh masuk database; jumlah baris = %d, ingin 2", n)
		}

		// SQL injection: payload harus tersimpan SEBAGAI TEKS, tabel tetap utuh.
		serang := `x'); DROP TABLE tasks; --`
		var s Task
		aman(t, func() { s, err = e.store.Add(Task{Judul: serang}) })
		if err != nil {
			t.Fatalf("judul berisi tanda kutip harus bisa disimpan apa adanya, dapat error: %v (jangan menyambung string SQL!)", err)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks`); n != 3 {
			t.Errorf("setelah payload injection jumlah baris = %d, ingin 3 (tabel rusak/terhapus?)", n)
		}
		if err := e.db.QueryRow(`SELECT judul FROM tasks WHERE id = $1`, s.ID).Scan(&judul); err != nil || judul != serang {
			t.Errorf("payload harus tersimpan persis sebagai teks: %q (err %v)", judul, err)
		}
	})

	t.Run("Level 4 - Get & List (rows ditutup, nol vs error)", func(t *testing.T) {
		e := siapkan(t, true)

		var kosong []Task
		var err error
		aman(t, func() { kosong, err = e.store.List() })
		if err != nil {
			t.Fatalf("List pada tabel kosong tidak boleh error: %v", err)
		}
		if kosong == nil || len(kosong) != 0 {
			t.Errorf("List pada tabel kosong harus []Task{} non-nil (supaya JSON menjadi [] bukan null), dapat %#v", kosong)
		}

		for i := 1; i <= 5; i++ {
			e.db.Exec(`INSERT INTO tasks (judul, selesai) VALUES ($1, $2)`, fmt.Sprintf("tugas-%d", i), i%2 == 0)
		}
		var daftar []Task
		aman(t, func() { daftar, err = e.store.List() })
		if err != nil || len(daftar) != 5 {
			t.Fatalf("List = %d tugas, err %v; ingin 5", len(daftar), err)
		}
		for i, tk := range daftar {
			if tk.Judul != fmt.Sprintf("tugas-%d", i+1) || tk.Selesai != ((i+1)%2 == 0) {
				t.Errorf("List[%d] = %+v, urutan harus menurut ID (ORDER BY id) dan kolom selesai terbaca benar", i, tk)
			}
			if i > 0 && tk.ID <= daftar[i-1].ID {
				t.Errorf("List harus berurutan menaik menurut ID")
			}
		}

		var got Task
		aman(t, func() { got, err = e.store.Get(daftar[3].ID) })
		if err != nil || got != daftar[3] {
			t.Errorf("Get(%d) = %+v, err %v; ingin %+v", daftar[3].ID, got, err, daftar[3])
		}
		aman(t, func() { _, err = e.store.Get(999999) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Get id yang tidak ada harus ErrTugasTidakDitemukan (dari sql.ErrNoRows, lewat errors.Is), dapat: %v", err)
		}

		// Pool berisi SATU koneksi: kalau rows/koneksi tidak dikembalikan (lupa rows.Close), panggilan berikutnya macet.
		e.db.SetMaxOpenConns(1)
		dalam(t, 20*time.Second, func() {
			for i := 0; i < 4; i++ {
				if _, err := e.store.List(); err != nil {
					t.Errorf("List ke-%d dengan pool 1 koneksi gagal: %v (rows tidak ditutup?)", i+1, err)
					return
				}
				if _, err := e.store.Get(daftar[0].ID); err != nil {
					t.Errorf("Get ke-%d dengan pool 1 koneksi gagal: %v", i+1, err)
					return
				}
				if _, err := e.store.Get(424242); !errors.Is(err, ErrTugasTidakDitemukan) {
					t.Errorf("Get tak ada ke-%d: %v", i+1, err)
					return
				}
			}
		})
		if inUse := e.db.Stats().InUse; inUse != 0 {
			t.Errorf("setelah semua panggilan selesai masih ada %d koneksi yang belum dikembalikan ke pool (rows.Close / defer)", inUse)
		}
	})

	t.Run("Level 5 - Update (RowsAffected, placeholder)", func(t *testing.T) {
		e := siapkan(t, true)
		a := Task{ID: seed(t, e.db, "A")}
		b := Task{ID: seed(t, e.db, "B")}

		var out Task
		var err error
		aman(t, func() { out, err = e.store.Update(Task{ID: a.ID, Judul: "  A baru ", Selesai: true}) })
		if err != nil {
			t.Fatalf("Update gagal: %v", err)
		}
		if out.ID != a.ID || out.Judul != "A baru" || !out.Selesai {
			t.Errorf("Update mengembalikan %+v, ingin ID %d, Judul \"A baru\", Selesai true", out, a.ID)
		}
		var judul string
		var selesai bool
		e.db.QueryRow(`SELECT judul, selesai FROM tasks WHERE id = $1`, a.ID).Scan(&judul, &selesai)
		if judul != "A baru" || !selesai {
			t.Errorf("perubahan harus tersimpan di database: judul=%q selesai=%v", judul, selesai)
		}
		e.db.QueryRow(`SELECT judul FROM tasks WHERE id = $1`, b.ID).Scan(&judul)
		if judul != "B" {
			t.Errorf("Update tidak boleh mengubah tugas lain (WHERE id = $n!): B menjadi %q", judul)
		}

		aman(t, func() { _, err = e.store.Update(Task{ID: 999999, Judul: "hantu"}) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Update id yang tidak ada harus ErrTugasTidakDitemukan (UPDATE tanpa baris terdampak TIDAK error di SQL -- cek RowsAffected / RETURNING), dapat: %v", err)
		}
		aman(t, func() { _, err = e.store.Update(Task{ID: b.ID, Judul: "  "}) })
		if !errors.Is(err, ErrInputKosong) {
			t.Errorf("Update judul kosong harus ErrInputKosong, dapat: %v", err)
		}
		e.db.QueryRow(`SELECT judul FROM tasks WHERE id = $1`, b.ID).Scan(&judul)
		if judul != "B" {
			t.Errorf("Update yang ditolak tidak boleh mengubah data: %q", judul)
		}

		// Injection lewat Update.
		serang := `x', selesai = true WHERE 1=1; --`
		aman(t, func() { _, err = e.store.Update(Task{ID: b.ID, Judul: serang}) })
		if err != nil {
			t.Fatalf("judul berisi tanda kutip harus tersimpan apa adanya: %v", err)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE selesai`); n != 1 {
			t.Errorf("hanya 1 tugas (A) yang selesai, tapi ada %d -- payload injection lolos?", n)
		}
		var s TaskStore = e.store
		if _, err := s.Get(b.ID); err != nil {
			t.Errorf("PostgresStore harus bisa dipakai lewat interface TaskStore: %v", err)
		}
	})

	t.Run("Level 6 - TambahBanyak dengan prepared statement", func(t *testing.T) {
		e := siapkan(t, true)

		var hasil []Task
		var err error
		aman(t, func() { hasil, err = e.store.TambahBanyak([]string{" satu ", "dua", "tiga"}) })
		if err != nil {
			t.Fatalf("TambahBanyak gagal: %v", err)
		}
		if len(hasil) != 3 || hasil[0].Judul != "satu" || hasil[1].Judul != "dua" || hasil[2].Judul != "tiga" {
			t.Fatalf("TambahBanyak harus mengembalikan 3 Task sesuai urutan masukan (judul di-Trim), dapat %+v", hasil)
		}
		if !(hasil[0].ID > 0 && hasil[0].ID < hasil[1].ID && hasil[1].ID < hasil[2].ID) {
			t.Errorf("ID harus terisi dan menaik: %d %d %d", hasil[0].ID, hasil[1].ID, hasil[2].ID)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks`); n != 3 {
			t.Errorf("jumlah baris di database = %d, ingin 3", n)
		}

		var kosong []Task
		aman(t, func() { kosong, err = e.store.TambahBanyak(nil) })
		if err != nil || kosong == nil || len(kosong) != 0 {
			t.Errorf("TambahBanyak(nil) harus mengembalikan slice kosong non-nil tanpa error, dapat %#v, %v", kosong, err)
		}

		aman(t, func() { _, err = e.store.TambahBanyak([]string{"ok", "  "}) })
		if !errors.Is(err, ErrInputKosong) {
			t.Errorf("judul kosong di dalam daftar harus ErrInputKosong, dapat: %v", err)
		}

		// Banyak baris: satu statement dipakai berulang kali.
		banyak := make([]string, 500)
		for i := range banyak {
			banyak[i] = "massal-" + strconv.Itoa(i)
		}
		var m []Task
		mulai := time.Now()
		dalam(t, 30*time.Second, func() { m, err = e.store.TambahBanyak(banyak) })
		if err != nil || len(m) != 500 {
			t.Fatalf("TambahBanyak 500 judul: %d hasil, err %v", len(m), err)
		}
		t.Logf("500 insert memakan %v", time.Since(mulai))

		// Heuristik: postgres.go harus benar-benar memakai Prepare (komentar tidak dihitung) dan menutup statement-nya.
		src := readFile("postgres.go")
		var kode []string
		for _, l := range strings.Split(src, "\n") {
			if i := strings.Index(l, "//"); i >= 0 {
				l = l[:i]
			}
			kode = append(kode, l)
		}
		bersih := strings.Join(kode, "\n")
		if !regexp.MustCompile(`\.Prepare(Context)?\(`).MatchString(bersih) {
			t.Error("TambahBanyak harus memakai prepared statement: db.Prepare / tx.Prepare (atau PrepareContext) -- tidak ditemukan di postgres.go")
		}
		if !regexp.MustCompile(`\.Close\(\)`).MatchString(bersih) {
			t.Error("statement/rows harus ditutup (defer stmt.Close()) -- tidak ditemukan pemanggilan Close() di postgres.go")
		}
	})

	t.Run("Level 7 - Delete + log dalam satu transaksi", func(t *testing.T) {
		e := siapkan(t, true)
		a := Task{ID: seed(t, e.db, "A")}
		b := Task{ID: seed(t, e.db, "B")}

		var err error
		aman(t, func() { err = e.store.Delete(a.ID) })
		if err != nil {
			t.Fatalf("Delete(%d) gagal: %v", a.ID, err)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE id = $1`, a.ID); n != 0 {
			t.Error("tugas harus terhapus dari tabel tasks")
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE id = $1`, b.ID); n != 1 {
			t.Error("Delete tidak boleh menghapus tugas lain")
		}
		var taskID int
		var aksi string
		var waktu time.Time
		if err := e.db.QueryRow(`SELECT task_id, aksi, waktu FROM task_logs`).Scan(&taskID, &aksi, &waktu); err != nil {
			t.Fatalf("harus ada TEPAT satu baris di task_logs setelah satu Delete: %v", err)
		}
		if taskID != a.ID || aksi != "hapus" {
			t.Errorf("baris log = task_id %d aksi %q, ingin task_id %d aksi \"hapus\"", taskID, aksi, a.ID)
		}
		if time.Since(waktu) > time.Minute || time.Until(waktu) > time.Minute {
			t.Errorf("kolom waktu log tidak masuk akal: %v", waktu)
		}

		// ID tidak ada: error, dan TIDAK ada log.
		aman(t, func() { err = e.store.Delete(999999) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Delete id yang tidak ada harus ErrTugasTidakDitemukan, dapat: %v", err)
		}
		aman(t, func() { err = e.store.Delete(a.ID) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Delete kedua kali harus ErrTugasTidakDitemukan, dapat: %v", err)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM task_logs`); n != 1 {
			t.Errorf("hapus yang gagal tidak boleh menulis log; jumlah baris task_logs = %d, ingin 1", n)
		}
	})

	t.Run("Level 8 - Rollback: semua atau tidak sama sekali", func(t *testing.T) {
		e := siapkan(t, true)
		a := Task{ID: seed(t, e.db, "A")}

		// Paksa INSERT log GAGAL lewat constraint di sisi database.
		if _, err := e.db.Exec(`ALTER TABLE task_logs ADD CONSTRAINT tolak_hapus CHECK (aksi <> 'hapus')`); err != nil {
			t.Fatal(err)
		}
		var err error
		aman(t, func() { err = e.store.Delete(a.ID) })
		if err == nil {
			t.Fatal("kalau INSERT log gagal, Delete harus mengembalikan error")
		}
		if errors.Is(err, ErrTugasTidakDitemukan) {
			t.Error("kegagalan menulis log bukan 'tugas tidak ditemukan' -- jangan menyamarkan error database")
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE id = $1`, a.ID); n != 1 {
			t.Error("Delete dan log harus SATU transaksi: log gagal -> DELETE harus ikut dibatalkan (Rollback), tapi tugas sudah hilang")
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM task_logs`); n != 0 {
			t.Errorf("tidak boleh ada log yang tertinggal, ada %d", n)
		}
		e.db.Exec(`ALTER TABLE task_logs DROP CONSTRAINT tolak_hapus`)

		// TambahBanyak harus atomik: satu judul buruk -> tidak ada yang masuk.
		aman(t, func() { _, err = e.store.TambahBanyak([]string{"ok1", "   ", "ok2"}) })
		if !errors.Is(err, ErrInputKosong) {
			t.Errorf("TambahBanyak dengan judul kosong harus ErrInputKosong, dapat: %v", err)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE judul LIKE 'ok%'`); n != 0 {
			t.Errorf("TambahBanyak harus atomik: ada %d baris 'ok*' tertinggal padahal ada judul yang ditolak", n)
		}
		// Kegagalan dari sisi database di tengah jalan (baris ke-2 melanggar CHECK).
		e.db.Exec(`ALTER TABLE tasks ADD CONSTRAINT tolak_boom CHECK (judul <> 'BOOM')`)
		aman(t, func() { _, err = e.store.TambahBanyak([]string{"sebelum", "BOOM", "sesudah"}) })
		if err == nil {
			t.Error("TambahBanyak harus error ketika database menolak salah satu baris")
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM tasks WHERE judul IN ('sebelum','BOOM','sesudah')`); n != 0 {
			t.Errorf("kegagalan di tengah TambahBanyak harus me-rollback SEMUANYA, tapi %d baris tertinggal", n)
		}

		// Kebersihan koneksi: dengan pool 1 koneksi, transaksi yang lupa di-Rollback menahan koneksi selamanya.
		e.db.SetMaxOpenConns(1)
		e.db.Exec(`ALTER TABLE task_logs ADD CONSTRAINT tolak_hapus CHECK (aksi <> 'hapus')`)
		dalam(t, 20*time.Second, func() {
			for i := 0; i < 3; i++ {
				e.store.Delete(a.ID)
				e.store.TambahBanyak([]string{"x", "BOOM"})
			}
			if _, err := e.store.List(); err != nil {
				t.Errorf("setelah beberapa transaksi gagal, List gagal: %v -- ada transaksi yang tidak di-Rollback (defer tx.Rollback()) sehingga koneksi tertahan", err)
			}
		})
		if inUse := e.db.Stats().InUse; inUse != 0 {
			t.Errorf("%d koneksi masih tertahan setelah transaksi gagal (lupa Rollback?)", inUse)
		}
	})

	t.Run("Level 9 - Timeout konteks & keamanan konkurensi", func(t *testing.T) {
		e := siapkan(t, true)
		a := Task{ID: seed(t, e.db, "A")}

		// Kunci tabel dari koneksi lain: query lain akan menunggu tanpa akhir kecuali ada timeout.
		lockDB := bukaDBUji(t, e.dsn)
		lockTx, err := lockDB.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lockTx.Exec(`LOCK TABLE tasks IN ACCESS EXCLUSIVE MODE`); err != nil {
			t.Fatal(err)
		}
		locked := true
		lepas := func() {
			if locked {
				locked = false
				lockTx.Rollback()
			}
		}
		t.Cleanup(lepas)

		lama := dbTimeout
		dbTimeout = 300 * time.Millisecond
		t.Cleanup(func() { dbTimeout = lama })

		ops := map[string]func() error{
			"List":   func() error { _, err := e.store.List(); return err },
			"Get":    func() error { _, err := e.store.Get(a.ID); return err },
			"Add":    func() error { _, err := e.store.Add(Task{Judul: "baru"}); return err },
			"Update": func() error { _, err := e.store.Update(Task{ID: a.ID, Judul: "ubah"}); return err },
			"Delete": func() error { return e.store.Delete(a.ID) },
		}
		for nama, op := range ops {
			var err error
			mulai := time.Now()
			dalam(t, 4*time.Second, func() { err = op() })
			if err == nil {
				t.Errorf("%s: tabel terkunci, operasi harus gagal karena timeout", nama)
				continue
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s: error harus membungkus context.DeadlineExceeded (pakai %%w), dapat: %v", nama, err)
			}
			if el := time.Since(mulai); el > 2*time.Second {
				t.Errorf("%s: baru menyerah setelah %v, padahal dbTimeout = 300ms (pakai variabel dbTimeout, bukan angka tetap)", nama, el)
			}
		}

		lepas()
		dalam(t, 5*time.Second, func() {
			if l, err := e.store.List(); err != nil || len(l) != 1 {
				t.Errorf("setelah kunci dilepas, store harus pulih: List = %d tugas, err %v (ada koneksi/transaksi yang tersangkut setelah timeout?)", len(l), err)
			}
		})
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM task_logs`); n != 0 {
			t.Errorf("Delete yang timeout tidak boleh meninggalkan log: %d baris", n)
		}

		// Konkurensi: server HTTP memanggil store dari banyak goroutine sekaligus.
		dbTimeout = lama
		const n = 40
		var wg sync.WaitGroup
		var mu sync.Mutex
		ids := map[int]bool{}
		var gagal []string
		wg.Add(n)
		dalam(t, 30*time.Second, func() {
			for i := 0; i < n; i++ {
				go func(i int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							mu.Lock()
							gagal = append(gagal, fmt.Sprint("panic: ", r))
							mu.Unlock()
						}
					}()
					tk, err := e.store.Add(Task{Judul: fmt.Sprintf("paralel-%d", i)})
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						gagal = append(gagal, err.Error())
						return
					}
					ids[tk.ID] = true
				}(i)
			}
			wg.Wait()
		})
		if len(gagal) > 0 {
			t.Errorf("%d dari %d Add paralel gagal, contoh: %s", len(gagal), n, gagal[0])
		}
		if len(ids) != n {
			t.Errorf("%d ID unik dari %d Add paralel, ingin %d", len(ids), n, n)
		}
		if l, _ := e.store.List(); len(l) != n+1 {
			t.Errorf("List = %d tugas, ingin %d", len(l), n+1)
		}
	})

	t.Run("Level 10 - Docker Compose, API di atas PostgreSQL & analisis", func(t *testing.T) {
		// --- docker-compose.yml ---
		var compose, nama string
		for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
			if c := readFile(n); c != "" {
				compose, nama = c, n
				break
			}
		}
		if compose == "" {
			t.Error("docker-compose.yml tidak ditemukan di folder pertemuan-07/ (tulis sesuai SOAL.md Level 10)")
		} else {
			var bersih []string
			for _, l := range strings.Split(compose, "\n") {
				if i := strings.Index(l, "#"); i >= 0 {
					l = l[:i]
				}
				bersih = append(bersih, l)
			}
			c := strings.Join(bersih, "\n")
			u, _ := url.Parse(baseDSN())
			user, pass := "", ""
			if u != nil && u.User != nil {
				user = u.User.Username()
				pass, _ = u.User.Password()
			}
			db := strings.TrimPrefix(u.Path, "/")
			port := u.Port()
			if port == "" {
				port = "5432"
			}
			if !regexp.MustCompile(`(?m)^\s*image:\s*["']?postgres:\d+`).MatchString(c) {
				t.Errorf("%s: image harus postgres dengan versi mayor yang dipatok, mis. `image: postgres:16` (bukan `latest`/tanpa tag)", nama)
			}
			if !regexp.MustCompile(`["']?` + regexp.QuoteMeta(port) + `\s*:\s*5432["']?`).MatchString(c) {
				t.Errorf("%s: publikasikan port host %s ke 5432 di container (ports: \"%s:5432\") supaya cocok dengan DSN", nama, port, port)
			}
			for k, v := range map[string]string{"POSTGRES_USER": user, "POSTGRES_PASSWORD": pass, "POSTGRES_DB": db} {
				if !regexp.MustCompile(k + `["']?\s*[:=]\s*["']?` + regexp.QuoteMeta(v)).MatchString(c) {
					t.Errorf("%s: %s harus diatur ke %q (sama dengan DSN yang dipakai aplikasi/test)", nama, k, v)
				}
			}
			if !regexp.MustCompile(`(?m)^\s*healthcheck:`).MatchString(c) || !strings.Contains(c, "pg_isready") {
				t.Errorf("%s: tambahkan healthcheck yang memakai pg_isready (agar `docker compose up --wait` menunggu database benar-benar siap)", nama)
			}
			if !regexp.MustCompile(`(?m)^\s*volumes:`).MatchString(c) {
				t.Errorf("%s: pasang volume untuk /var/lib/postgresql/data supaya data tidak hilang saat container dibuat ulang", nama)
			}
			if regexp.MustCompile(`(?m)^version:`).MatchString(c) {
				t.Errorf("%s: hapus baris `version:` di paling atas -- Compose modern menganggapnya obsolete dan mencetak peringatan", nama)
			}
		}

		// --- API di atas PostgreSQL: handler tidak berubah, hanya store yang ditukar ---
		e := siapkan(t, false)
		var err error
		aman(t, func() { err = e.store.Migrate() })
		if err != nil {
			t.Fatalf("Migrate gagal: %v", err)
		}
		var h http.Handler
		aman(t, func() { h = NewAPI(e.store).Routes() })
		kirim := func(method, path, body string) *httptest.ResponseRecorder {
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
		if rec := kirim("GET", "/tasks", ""); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("GET /tasks pada database kosong: %d %q, ingin 200 \"[]\"", rec.Code, rec.Body.String())
		}
		rec := kirim("POST", "/tasks", `{"judul":"lewat postgres"}`)
		var dibuat Task
		json.Unmarshal(rec.Body.Bytes(), &dibuat)
		if rec.Code != 201 || dibuat.ID == 0 || dibuat.Judul != "lewat postgres" {
			t.Fatalf("POST /tasks: %d %s", rec.Code, rec.Body.String())
		}
		path := fmt.Sprintf("/tasks/%d", dibuat.ID)
		if rec := kirim("PUT", path, `{"judul":"diubah","selesai":true}`); rec.Code != 200 {
			t.Errorf("PUT: %d %s", rec.Code, rec.Body.String())
		}
		if rec := kirim("GET", path, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"diubah"`) || !strings.Contains(rec.Body.String(), `"selesai":true`) {
			t.Errorf("GET setelah PUT: %d %s", rec.Code, rec.Body.String())
		}
		if rec := kirim("POST", "/tasks", `{"judul":"   "}`); rec.Code != 400 {
			t.Errorf("judul kosong tetap harus ditolak -> %d, ingin 400", rec.Code)
		}
		if rec := kirim("GET", "/tasks/999999", ""); rec.Code != 404 {
			t.Errorf("GET id tidak ada: %d, ingin 404", rec.Code)
		}
		// Data bertahan: store baru di atas database yang sama melihat data yang sama.
		h2 := NewAPI(NewPostgresStore(e.db)).Routes()
		rec2 := httptest.NewRecorder()
		h2.ServeHTTP(rec2, httptest.NewRequest("GET", path, nil))
		if rec2.Code != 200 {
			t.Errorf("data harus bertahan di database, bukan di memori proses: %d", rec2.Code)
		}
		if rec := kirim("DELETE", path, ""); rec.Code != 204 {
			t.Errorf("DELETE: %d", rec.Code)
		}
		if n := hitung(t, e.db, `SELECT COUNT(*) FROM task_logs WHERE task_id = $1 AND aksi = 'hapus'`, dibuat.ID); n != 1 {
			t.Errorf("DELETE lewat API harus menulis log 'hapus': %d baris", n)
		}

		// List() yang gagal harus menjadi 500 generik (bukan 200 berisi null).
		hl := NewAPI(errListStore{NewMemoryStore()}).Routes()
		var recL *httptest.ResponseRecorder
		aman(t, func() {
			recL = httptest.NewRecorder()
			hl.ServeHTTP(recL, httptest.NewRequest("GET", "/tasks", nil))
		})
		if recL.Code != 500 {
			t.Errorf("store.List() error -> GET /tasks harus 500, dapat %d (%s) -- jangan membuang error dengan `_`", recL.Code, strings.TrimSpace(recL.Body.String()))
		} else if !strings.Contains(recL.Body.String(), "terjadi kesalahan internal") || strings.Contains(recL.Body.String(), "rahasia") {
			t.Errorf("body 500 harus generik dan tidak membocorkan detail: %s", recL.Body.String())
		}
		// Database benar-benar mati -> 500 generik juga.
		e.db.Close()
		if rec := kirim("GET", "/tasks", ""); rec.Code != 500 || strings.Contains(strings.ToLower(rec.Body.String()), "sql:") {
			t.Errorf("database tertutup: GET /tasks = %d %s, ingin 500 generik", rec.Code, rec.Body.String())
		}

		// --- Analisis README ---
		s := section(readFile("../README.md"), "Analisis Pertemuan 7")
		if !filled(s, 250) {
			t.Fatalf("section '## Analisis Pertemuan 7' di README.md (root repo) tidak ada atau belum diisi memadai (minimal 250 karakter, saat ini %d) -- tambahkan heading persis itu kalau belum ada", len(strings.TrimSpace(s)))
		}
		if !mengandungSalahSatu(s, "transaksi", "transaction") {
			t.Error("analisis harus membahas transaksi")
		}
		if !mengandungSalahSatu(s, "rollback", "atomik", "atomic", "semua atau tidak", "semuanya atau") {
			t.Error("analisis harus menyebut rollback/atomik (semua-atau-tidak-sama-sekali)")
		}
		if !mengandungSalahSatu(s, "healthcheck", "health check", "siap", "ready") {
			t.Error("analisis harus membahas healthcheck / kesiapan database saat container baru menyala")
		}
	})
}
