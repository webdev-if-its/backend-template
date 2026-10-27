# Pertemuan 9 — Task Manager v4: Middleware, Validasi & Penguatan REST API

REST API Pertemuan 6 sudah jalan, tapi belum "tahan banting": tidak ada log, satu panic mematikan respons, input dari klien dipercaya begitu saja. Minggu ini kita memperkuatnya dengan **middleware** dan **validasi berlapis**. Ini juga latihan terintegrasi — semua yang dipakai (routing, JSON, error, interface, `defer/recover`, goroutine per request) sudah kalian pelajari di pertemuan sebelumnya. Penyimpanan tetap **di memori** (`MemoryStore`); tugas ini **tidak memakai database** sama sekali, jadi tidak ada yang perlu diinstal atau dijalankan selain Go.

## Struktur folder

```
pertemuan-09/
├── go.mod          # sudah ada ('go 1.22' -- jangan diturunkan)
├── store.go        # SUDAH LENGKAP, jangan diubah (Task, TaskStore, MemoryStore)
├── api.go          # API Pertemuan 6 yang sudah jalan -- kalian EDIT di dua tempat bertanda TODO (Level 8 & 9)
├── middleware.go   # <-- KERJAKAN: Chain, statusRecorder, Logging, Recover, RequireJSON, NewHandler
├── validasi.go     # <-- KERJAKAN: ValidationError, ValidasiTugas
├── main.go         # sudah ada (memakai NewHandler)
└── main_test.go    # test grading, jangan diubah
```

Test memakai `httptest` (tidak membuka port) dan **menangkap keluaran paket `log`** untuk memeriksa apa yang kalian catat — jadi gunakan `log.Printf` (paket `log` standar), bukan `fmt.Println`. Untuk mencoba sungguhan: `go run .` lalu `curl.exe` (PowerShell: `curl.exe`, bukan `curl`).

Ada satu section README yang dicek di Level 10: heading persis `## Analisis Pertemuan 9` — **tambahkan sendiri** di `README.md` root repo kalian.

---

## Level 1 — `Logging`: satu baris per request

```go
type Middleware func(http.Handler) http.Handler
func Logging(next http.Handler) http.Handler
```

Untuk setiap request, cetak **tepat satu baris** lewat `log.Printf("%s %s %d %v", method, path, status, durasi)`, contoh: `GET /tasks/5 404 132µs`. `method` dari `r.Method`, `path` dari `r.URL.Path`. Respons handler harus diteruskan **apa adanya** (status, header, body). Agar status bisa dicatat, bungkus `http.ResponseWriter` dengan `statusRecorder` (struct yang meng-embed `http.ResponseWriter` dan menyimpan `status`; **override** `WriteHeader` untuk mengingat kodenya sebelum meneruskannya).

## Level 2 — `Logging`: status default, panggilan pertama, durasi

Lengkapi `statusRecorder`/`Logging`:

- Handler yang **tidak pernah memanggil `WriteHeader`** (hanya `w.Write`) tetap menghasilkan 200 — jadi status awal `statusRecorder` harus 200.
- `WriteHeader` bisa terpanggil lebih dari sekali; Go hanya mengirim yang **pertama**. Log harus mencatat status yang **benar-benar terkirim ke klien** (panggilan pertama), bukan yang terakhir.
- Kolom durasi harus berupa `time.Duration` yang bisa di-parse (`%v` pada `time.Since(mulai)`) dan mencerminkan waktu handler sebenarnya (test menidurkan handler 30 ms dan mengharapkan durasi ≥ 25 ms).

## Level 3 — `Chain`

```go
func Chain(h http.Handler, mws ...Middleware) http.Handler
```

Bungkus `h` dengan semua `mws`. **Middleware pertama di daftar adalah yang paling luar** (jalan paling dulu saat request masuk, paling akhir saat respons keluar): `Chain(h, A, B, C)` menghasilkan urutan `A> B> C> h <C <B <A`. Tanpa middleware, hasilnya setara `h`. Middleware yang tidak memanggil `next` menghentikan rantai di situ. Petunjuk: bungkus dari daftar **paling belakang** ke depan.

## Level 4 — `Recover`: panic jadi 500

```go
func Recover(next http.Handler) http.Handler
```

`defer` + `recover()`: kalau handler di bawahnya panic, balas **500** dengan `Content-Type: application/json` dan body `{"error":"terjadi kesalahan internal"}` (pakai `writeJSON` + `errorBody` yang sudah ada), dan **server tetap hidup** untuk request berikutnya. Nilai panic bisa apa saja (`string`, `error`, struct, angka). Handler yang tidak panic **tidak boleh diubah sama sekali**.

## Level 5 — `Recover`: log lengkap, klien tidak melihat apa-apa

Saat menangkap panic, catat ke **log**: nilai panic **dan stack trace** (`runtime/debug.Stack()`) — tanpa stack trace kalian tahu *bahwa* ada panic tapi tidak tahu *di mana*. Body respons ke klien **tidak boleh** memuat pesan panic, stack trace, atau nama file (itu informasi internal yang berguna bagi penyerang).

## Level 6 — `Logging` + `Recover`: request yang panic tetap tercatat 500

Dengan susunan `Logging(Recover(h))` (Recover di **dalam** Logging), request yang panic harus tetap muncul di log sebagai `GET /boom 500 <durasi>` — ini terjadi otomatis kalau `Logging` membungkus `Recover` dan `statusRecorder` mencatat `WriteHeader(500)` yang dilakukan `Recover`. (Kalau urutannya dibalik, panic naik melewati `Logging` dan barisnya hilang — coba sendiri, ini bahan analisis Level 10.)

Kasus sulit: handler sudah mengirim `WriteHeader(200)` + sebagian body, **baru** panic. Klien sudah terlanjur menerima 200; `Recover` tetap mencoba menulis 500 (Go menolak dan mencetak peringatan `superfluous response.WriteHeader`). Log harus jujur mencatat **200**, bukan 500 — dan itulah kenapa `statusRecorder` hanya boleh mengingat panggilan `WriteHeader` **pertama**.

## Level 7 — `ValidasiTugas`

```go
type ValidationError struct{ Fields map[string]string }
func (e *ValidationError) Error() string          // "validasi gagal"
func ValidasiTugas(in taskInput) error
```

Aturan judul: setelah `strings.TrimSpace`, tidak boleh kosong (`"tidak boleh kosong"`) dan **maksimal 100 karakter** (`"maksimal 100 karakter"`). Hitung **karakter, bukan byte** (`utf8.RuneCountInString`): 100 huruf `あ` adalah 300 byte tetapi hanya 100 karakter, dan harus **lolos**; 101 harus ditolak. Input valid mengembalikan `nil`; input tidak valid mengembalikan `*ValidationError` dengan `Fields["judul"]` berisi alasannya. `Selesai` tidak divalidasi.

Jebakan yang dicek: kalau kalian mendeklarasikan `var ve *ValidationError` lalu `return ve` di jalur sukses, `err != nil` akan bernilai **true** (nilai `nil` bertipe pointer yang dibungkus interface tidak sama dengan `nil` interface). Kembalikan `nil` secara eksplisit.

## Level 8 — Validasi di API: 400 + `detail`

Edit `api.go` (bagian TODO Level 8): `createTask` dan `updateTask` memanggil `ValidasiTugas(in)` **sebelum** menyentuh store; kegagalannya diteruskan ke `writeError`, yang kalian lengkapi dengan `errors.As` (bukan `==`, supaya error yang dibungkus `%w` tetap terbaca):

```json
HTTP 400
{"error":"validasi gagal","detail":{"judul":"tidak boleh kosong"}}
```

`detail` hanya muncul untuk kegagalan validasi (tag `omitempty` pada `errorBody.Detail` sudah ada di starter) — error lain (mis. 404) tetap `{"error":"tugas tidak ditemukan"}` **tanpa** kunci `detail`. Request yang ditolak tidak boleh mengubah isi store. Tiga skenario dicek: judul kosong/spasi, judul 101 karakter (juga 101 huruf multibyte), dan judul 100 huruf multibyte yang harus diterima (201).

## Level 9 — Lapisan luar: `RequireJSON`, batas ukuran, field asing, salah tipe

Validasi bisnis (Level 8) hanya masuk akal kalau **bentuk** request-nya sudah benar. Lapisan-lapisan luar ini menjaga itu:

**`RequireJSON`** (middleware): untuk `POST`/`PUT`/`PATCH`, `Content-Type` harus `application/json` — kalau tidak, jawab **415** dengan `{"error":"Content-Type harus application/json"}` dan **jangan** panggil handler. Parse dengan `mime.ParseMediaType`, sehingga `application/json; charset=utf-8` dan `Application/JSON` tetap diterima sementara `application/jsonx`, `text/plain`, dan header kosong ditolak. `GET`/`DELETE` (tanpa body) lolos tanpa syarat.

**`decodeInput`** di `api.go` (bagian TODO Level 9) — bedakan kegagalan dengan pesan dan status yang **persis** ini:

| Kondisi | Status | `error` |
|---|---|---|
| body lebih dari `maxBodyBytes` (1024) — pakai `http.MaxBytesReader`, deteksi `*http.MaxBytesError` | **413** | `body terlalu besar` |
| ada field yang tidak dikenal (`Decoder.DisallowUnknownFields()`; deteksi lewat awalan pesan `json: unknown field`) | 400 | `body mengandung field yang tidak dikenal` |
| tipe field salah, mis. `{"judul":123}` (`*json.UnmarshalTypeError`) | 400 | `field "judul" harus bertipe string` (untuk `selesai: "ya"` → `field "selesai" harus bertipe bool`) |
| bukan JSON / kosong / terpotong | 400 | `body bukan JSON yang valid` |

Akibat sampingan yang disengaja: `{"id":5,"judul":"x"}` sekarang **ditolak** (field asing), bukan diam-diam diabaikan seperti di Pertemuan 6 — lebih baik gagal keras daripada membiarkan klien mengira ID-nya dipakai. Body sah yang dipadatkan spasi hingga ±900 byte tetap diterima. Request yang ditolak tidak menyimpan apa pun; `PUT` memakai lapisan yang sama.

## Level 10 — `NewHandler` & Analisis (README)

```go
func NewHandler(store TaskStore) http.Handler
```

Susun semuanya menjadi satu handler: `Chain(NewAPI(store).Routes(), Logging, Recover, RequireJSON)` — dipasang **sekali** pada **seluruh mux**, bukan per-route (`main.go` sudah memakainya). Test memeriksa lewat `NewHandler`:

- request biasa tercatat di log;
- `POST` dengan `text/plain` → **415 dan tercatat** `POST /tasks 415` (artinya `Logging` di luar `RequireJSON`);
- rute yang tidak ada (`GET /nope`) → 404 dan **tercatat** (rantai membungkus seluruh mux — kalau kalian memasangnya per-route, baris ini tidak akan muncul);
- panic di lapisan store → 500 JSON generik, tercatat `GET /tasks/1 500` (artinya `Recover` di dalam `Logging`), dan request berikutnya tetap dilayani.

Lalu isi `## Analisis Pertemuan 9` di `README.md` (root repo): dengan kata-kata sendiri, **minimal ±200 karakter**, jawab:

1. Kenapa `Recover` harus berada **di dalam** `Logging` pada rantai, dan apa yang terjadi di log kalau urutannya dibalik? Jelaskan lewat urutan eksekusi (siapa yang "menunggu" siapa), bukan sekadar "begitu aturannya".
2. Server `net/http` memproses tiap request di goroutine sendiri. Apakah `Recover` yang kalian tulis akan menyelamatkan server kalau handler membuat goroutine baru (`go func(){ ... panic ... }()`) dan panic di dalamnya? Kenapa / kenapa tidak?

**Dicek otomatis:** section terisi (≥ 200 karakter, bukan placeholder), menyebut *recover*, *goroutine*, dan salah satu dari *urutan/luar/dalam/order*. Ini hanya memastikan kalian benar-benar menjawab — kualitas penalarannya dibaca dosen.
