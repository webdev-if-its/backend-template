# Pertemuan 10 — Task Manager v3: TaskStore di atas PostgreSQL

Momen pembuktian kenapa kita membuat interface `TaskStore` sejak Pertemuan 4: minggu ini data Task Manager pindah dari memori ke **PostgreSQL** — dan REST API dari Pertemuan 6 (routing, JSON, error terstruktur) hampir **tidak berubah**. Kalian hanya menulis implementasi `TaskStore` yang baru: `PostgresStore`.

Satu perubahan kontrak yang **memang harus terjadi**: `List()` sekarang `([]Task, error)`. Database bisa putus; kalau `List()` tidak bisa melaporkan kegagalan, error-nya terpaksa ditelan. Perubahan interface ini merambat ke `MemoryStore` (sudah kami sesuaikan) dan ke `listTasks` di `api.go` (bagian kalian, Level 10).

## Struktur folder

```
pertemuan-10/
├── go.mod / go.sum    # sudah ada (driver pgx v5.7.6; butuh Go >= 1.23)
├── store.go           # SUDAH LENGKAP: Task, TaskStore (List -> ([]Task, error)), MemoryStore
├── api.go             # REST API Pertemuan 6 -- kalian hanya menyentuh listTasks (Level 10)
├── postgres.go        # <-- YANG KALIAN KERJAKAN
├── main.go            # sudah ada: membaca DATABASE_URL, Connect, Migrate, menyalakan server
├── docker-compose.yml # <-- KALIAN TULIS (Level 10) -- belum ada
└── main_test.go       # test grading, jangan diubah
```

## Menyiapkan database untuk test

**Semua level butuh PostgreSQL yang menyala** (ini bukan soal yang bisa dinilai tanpa database). Test membaca `DATABASE_URL`; kalau kosong dipakai:

```
postgres://taskmanager:rahasia@localhost:5433/taskmanager?sslmode=disable
```

Cara paling mudah: tulis `docker-compose.yml` (spesifikasinya di Level 10 — tidak apa-apa mengerjakannya paling awal), lalu:

```bash
docker compose up -d --wait
go test ./... -v
```

Kalau database tidak bisa dihubungi, level akan gagal dengan pesan yang menjelaskan DSN mana yang dicoba. Docker Desktop di Windows kadang butuh beberapa menit untuk siap pertama kali — cek jauh sebelum tenggat, jangan di menit terakhir. Yang tidak mau memakai Docker boleh memasang PostgreSQL lokal dan mengarahkan `DATABASE_URL` ke sana (tapi Level 10 tetap memeriksa `docker-compose.yml`).

**Isolasi:** setiap level bekerja di **schema sementara** miliknya sendiri (lewat `search_path`) dan dihapus setelahnya; data kalian di schema `public` tidak tersentuh, dan banyak orang bisa berbagi satu server tanpa saling mengganggu. Konsekuensi untuk kode kalian: **tulis nama tabel tanpa awalan schema** (`tasks`, bukan `public.tasks`), dan buat tabel lewat `Migrate` yang memakai nama polos.

**Aturan untuk semua method di `postgres.go`** (dicek lewat perilaku, bukan sekadar dibaca):

- nilai dari luar **selalu** lewat placeholder `$1, $2, …` — jangan pernah menyambung string SQL (test menyerang dengan payload `x'); DROP TABLE tasks; --`);
- tiap operasi memakai `context` dengan timeout **`dbTimeout`** (variabel di starter — pakai variabelnya, bukan angka tetap);
- error dari database dibungkus dengan `%w` dan konteks (`fmt.Errorf("add task: %w", err)`), sementara `ErrTugasTidakDitemukan`/`ErrInputKosong` dikembalikan apa adanya supaya `writeError` (Pertemuan 6) tetap bekerja.

---

## Level 1 — `Connect`

```go
func Connect(dsn string) (*sql.DB, error)
```

`sql.Open("pgx", dsn)` **tidak pernah memeriksa koneksi** — ia hanya menyiapkan pool. Dengan password salah atau database tidak ada, `sql.Open` tetap sukses dan errornya baru muncul di query pertama. Karena itu `Connect` harus melanjutkan dengan `PingContext` (timeout `dbTimeout`); kalau gagal, tutup `db` dan kembalikan error yang dibungkus `%w`. Atur pool: `SetMaxOpenConns(10)` (dicek test lewat `db.Stats()`), `SetMaxIdleConns(5)`, `SetConnMaxLifetime(30 * time.Minute)`.

Dicek: DSN benar → `db` yang bisa di-`Ping` dan `MaxOpenConnections == 10`; database yang tidak ada, port yang tidak didengarkan siapa pun, dan DSN sampah → **error** (bukan `db` yang kelihatan sukses).

## Level 2 — `Migrate`

```go
func (s *PostgresStore) Migrate() error
```

Buat dua tabel dengan `CREATE TABLE IF NOT EXISTS` (idempotent — aman dipanggil di setiap start aplikasi, dan **tidak boleh** `DROP` apa pun: data harus selamat dari `Migrate` kedua). Skema **persis**:

```sql
tasks(
  id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  judul   TEXT NOT NULL,
  selesai BOOLEAN NOT NULL DEFAULT FALSE
)
task_logs(
  id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  task_id INT NOT NULL,
  aksi    TEXT NOT NULL,
  waktu   TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

Test membaca `information_schema.columns` dan memeriksa nama kolom, tipe, `NOT NULL`, identity, dan default. Kolom tambahan tidak diperbolehkan (jumlah kolom dicek). `task_logs` sengaja **tanpa** foreign key ke `tasks`: log penghapusan harus tetap ada setelah barisnya dihapus.

## Level 3 — `Add`

```go
func (s *PostgresStore) Add(t Task) (Task, error)
```

Judul di-`TrimSpace`; kosong → `ErrInputKosong` **tanpa menyentuh database**. Simpan dengan `INSERT ... RETURNING id` (driver pgx tidak mendukung `LastInsertId`) dan kembalikan `Task` lengkap dengan ID dari database. `Selesai` ikut tersimpan. Judul berisi tanda kutip atau payload injection harus tersimpan **apa adanya sebagai teks** dan tabel tetap utuh.

## Level 4 — `Get` & `List`

```go
func (s *PostgresStore) Get(id int) (Task, error)
func (s *PostgresStore) List() ([]Task, error)
```

`Get`: `sql.ErrNoRows` → `ErrTugasTidakDitemukan` (deteksi dengan `errors.Is`, jangan bandingkan teks error). `List`: `ORDER BY id`, hasil kosong = `[]Task{}` **non-nil** (supaya JSON `[]`, bukan `null`); periksa `rows.Err()` setelah loop dan tutup `rows` (`defer rows.Close()`).

Kenapa test ini menurunkan pool jadi **satu koneksi** lalu memanggil `List`/`Get` berulang: query yang koneksinya tidak dikembalikan ke pool (rows tidak ditutup, transaksi menggantung) membuat panggilan berikutnya menunggu tanpa akhir — dengan pool 1 koneksi kebocoran itu langsung ketahuan sebagai macet/`InUse > 0`.

## Level 5 — `Update`

```go
func (s *PostgresStore) Update(t Task) (Task, error)
```

Ganti `judul` (di-Trim) dan `selesai` tugas ber-ID `t.ID`; kembalikan tugas terbaru. Judul kosong → `ErrInputKosong` (data tidak berubah). **Jebakan:** `UPDATE ... WHERE id = $3` pada ID yang tidak ada **bukan error di SQL** — ia sukses dengan 0 baris terdampak. Periksa `RowsAffected()` (atau pakai `RETURNING`) dan kembalikan `ErrTugasTidakDitemukan`. Tugas lain tidak boleh tersentuh (test menyerang dengan judul `x', selesai = true WHERE 1=1; --`).

## Level 6 — `TambahBanyak` dengan prepared statement

```go
func (s *PostgresStore) TambahBanyak(judul []string) ([]Task, error)
```

Simpan banyak judul sekaligus: `Prepare` **sekali**, jalankan berkali-kali. Kembalikan `[]Task` sesuai urutan masukan (judul di-Trim, ID menaik); daftar kosong → `[]Task{}` non-nil tanpa error; judul kosong di tengah daftar → `ErrInputKosong`. Test memakai 500 judul.

Yang dicek otomatis di level ini: hasilnya benar, **dan** `postgres.go` memang memuat pemanggilan `.Prepare(`/`.PrepareContext(` (baris komentar tidak dihitung) serta `Close()` untuk statement-nya. Ini heuristik pada teks kode — bukan bukti bahwa prepared statement memang lebih cepat. Di kelas kita sudah mengukur: untuk operasi sederhana **tidak ada percepatan yang konsisten**; parameterisasi (`$1`) sudah cukup untuk keamanan, sedangkan `Prepare` baru berguna untuk statement yang sama berulang-ulang. Latihan ini sengaja menyediakan kasus itu.

*(Atomisitas `TambahBanyak` — semua-atau-tidak-sama-sekali — baru dituntut di Level 8; di level ini cukup jalur suksesnya benar.)*

## Level 7 — `Delete` + log dalam satu transaksi

```go
func (s *PostgresStore) Delete(id int) error
```

Hapus tugas **dan** catat aksinya di `task_logs` (`INSERT INTO task_logs (task_id, aksi) VALUES ($1, 'hapus')`) — dua statement yang harus **sama-sama berhasil atau sama-sama batal**, jadi jalankan dalam **satu transaksi** (`BeginTx` → `tx.ExecContext` ×2 → `Commit`). ID tidak ada (`RowsAffected() == 0`) → `ErrTugasTidakDitemukan` dan **tidak ada** baris log yang ditulis; menghapus dua kali → error kedua kalinya, log tetap satu.

Dicek: setelah satu `Delete` sukses ada **tepat satu** baris `task_logs` (`task_id`, `aksi = 'hapus'`, `waktu` masuk akal); tugas lain tidak ikut terhapus.

## Level 8 — Rollback: semua atau tidak sama sekali

Test **memaksa kegagalan dari sisi database** lewat constraint, lalu memeriksa tidak ada yang setengah jadi:

1. Constraint `CHECK (aksi <> 'hapus')` dipasang di `task_logs` → `INSERT` log gagal → `Delete` harus mengembalikan error (bukan `ErrTugasTidakDitemukan`) **dan tugasnya harus masih ada** — `DELETE` yang sudah dijalankan di dalam transaksi harus ikut dibatalkan.
2. `TambahBanyak` menjadi **atomik**: `[]string{"ok1", "   ", "ok2"}` → `ErrInputKosong` dan tidak ada baris yang masuk; constraint `CHECK (judul <> 'BOOM')` di `tasks` dengan `[]string{"sebelum", "BOOM", "sesudah"}` → error dan **nol** baris tertinggal (baris "sebelum" yang sudah sempat di-INSERT harus batal). Bungkus seluruh `TambahBanyak` dalam satu transaksi.
3. Kebersihan koneksi: dengan pool 1 koneksi, setelah beberapa `Delete`/`TambahBanyak` yang gagal `List()` **tidak boleh macet** — transaksi yang gagal harus melepas koneksinya. Pola aman: `defer tx.Rollback()` tepat setelah `BeginTx` (memanggil `Rollback` setelah `Commit` sukses tidak berbahaya — hasilnya hanya `sql.ErrTxDone`).

## Level 9 — Timeout konteks & keamanan konkurensi

Test **mengunci tabel** `tasks` dari koneksi lain (`LOCK TABLE ... ACCESS EXCLUSIVE`) — dari sudut pandang aplikasi ini sama dengan database yang macet — dan menurunkan `dbTimeout` menjadi 300 ms. Kelima operasi (`List`, `Get`, `Add`, `Update`, `Delete`) harus:

- **gagal dalam waktu singkat** (jauh di bawah 2 detik), bukan menunggu kunci dilepas selamanya;
- mengembalikan error yang **membungkus `context.DeadlineExceeded`** (`errors.Is` bernilai true — artinya kalian membungkus dengan `%w`, bukan `%v`);
- tidak meninggalkan efek samping (mis. log dari `Delete` yang timeout).

Setelah kunci dilepas, store harus **pulih** (tidak ada koneksi/transaksi yang tersangkut). Terakhir, 40 goroutine memanggil `Add` bersamaan (server HTTP memang melayani tiap request di goroutine sendiri) — semuanya harus sukses dengan 40 ID berbeda. Ini jaminan yang diberikan `database/sql` (pool aman untuk konkurensi) asal kalian tidak menyimpan state bersama di struct `PostgresStore`.

Bahan diskusi (tidak dicek otomatis): kenapa `dbTimeout` sengaja *variabel* di starter, bukan `const`? Dan kenapa timeout di sisi aplikasi tidak menggantikan kebutuhan akan timeout/`statement_timeout` di sisi database?

## Level 10 — Docker Compose, API di atas PostgreSQL & Analisis (README)

**1. `docker-compose.yml`** (di folder `pertemuan-10/`) — tulis sendiri; dicek memuat:

- `image: postgres:<versi mayor>` (mis. `postgres:16` — versi dipatok, **bukan** `latest`/tanpa tag; alasannya: `latest` bisa melompat versi mayor dan merusak data di volume);
- `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` yang **sama dengan DSN** (`taskmanager` / `rahasia` / `taskmanager` kalau memakai default) dan `ports` yang memetakan port host di DSN (`5433`) ke `5432`;
- `healthcheck` yang memakai `pg_isready`;
- `volumes` untuk `/var/lib/postgresql/data`;
- **tanpa** baris `version:` di paling atas (Compose modern menganggapnya obsolete dan mencetak peringatan).

Jalankan dengan `docker compose up -d --wait` — opsi `--wait` menunggu healthcheck; tanpa itu `up -d` kembali segera padahal PostgreSQL di dalamnya belum siap menerima koneksi.

**2. API di atas PostgreSQL** — `NewAPI(NewPostgresStore(db)).Routes()`: seluruh CRUD REST (POST/GET/PUT/DELETE, judul kosong → 400, 404) berjalan di atas database, data bertahan di database (store baru di atas `db` yang sama melihat data yang sama), dan `DELETE` lewat API menulis log. Yang perlu kalian ubah di `api.go`: **`listTasks`** — baris `tugas, _ := a.store.List()` membuang error; teruskan ke `writeError` supaya kegagalan database menjadi **500 generik** `{"error":"terjadi kesalahan internal"}` (tanpa detail), bukan `200` dengan `null`. Test memeriksa dua skenario: store yang `List()`-nya sengaja gagal, dan database yang benar-benar ditutup.

**3. `## Analisis Pertemuan 10`** di `README.md` (root repo) — dengan kata-kata sendiri, **minimal ±250 karakter**:

1. Kenapa `Delete` + `INSERT` log harus **satu transaksi**? Apa yang terjadi pada data kalau keduanya dijalankan sebagai dua statement terpisah dan yang kedua gagal?
2. `docker compose up -d` selesai dalam sekejap tapi koneksi pertama ke database bisa gagal. Kenapa? Bagaimana `healthcheck` + `--wait` menolong?

**Dicek otomatis:** README terisi (≥ 250 karakter, bukan placeholder), menyebut *transaksi*, salah satu dari *rollback/atomik/semua atau tidak*, dan *healthcheck/siap/ready*. Ini hanya memastikan kalian benar-benar menjawab — kualitas penalarannya dibaca dosen.
