# Pertemuan 4 — Task Manager v1: Struct, Interface & Generics

Lanjutan proyek Task Manager. Minggu lalu datanya berupa `TokoTugas` + fungsi lepas; sekarang kita modelkan domainnya dengan **struct + method**, lalu kita definisikan **interface `TaskStore`** dan satu implementasi in-memory-nya. Interface ini bukan hiasan: di Pertemuan 6 handler HTTP akan bicara ke `TaskStore` saja, dan di Pertemuan 9 kita mengganti isinya dengan PostgreSQL tanpa menyentuh handler.

Semua yang dicek ada di `pertemuan-04/main.go` — starter sudah berisi kerangka lengkap (struct, interface, semua signature) dengan `panic("belum diimplementasikan")`. **Jangan mengubah signature yang sudah ada** — test memanggilnya persis seperti tertulis. `TaskStore` (interface) juga jangan diubah; yang kalian tulis adalah implementasinya.

Ada satu section README yang ikut dicek di Level 10 (heading persis `## Analisis Pertemuan 4`). Repo kalian dibuat dari template di awal semester, jadi heading ini belum ada di `README.md` kalian — **tambahkan sendiri** di root repo (bukan di dalam folder `pertemuan-04/`).

---

## Level 1 — `NewTask`

```go
func NewTask(judul string) (Task, error)
```

Buat `Task` baru: `Judul` diisi dari `judul` **setelah spasi di kedua ujung dibuang** (`strings.TrimSpace`), `Selesai` false. Kalau judul kosong atau hanya berisi spasi/tab/newline, kembalikan `Task{}` dan `ErrInputKosong`. (`ID` diisi oleh store nanti, bukan di sini; `CreatedAt`/`UpdatedAt` ada di Level 9.)

## Level 2 — `(*Task).MarkDone`

```go
func (t *Task) MarkDone()
```

Ubah `Selesai` menjadi `true`. Perhatikan **receiver-nya pointer** (`*Task`): test mengecek bahwa `tugas[1].MarkDone()` pada elemen slice benar-benar mengubah elemen itu, dan bahwa `MarkDone` **tidak** ada di method set tipe `Task` (nilai). Memanggilnya dua kali harus aman (tetap `true`).

## Level 3 — `(*Task).Rename`

```go
func (t *Task) Rename(judul string) error
```

Ganti judul (spasi ujung dibuang). Kalau judul baru kosong/spasi: kembalikan `ErrInputKosong` dan **judul lama tetap utuh** — method yang gagal tidak boleh meninggalkan objek setengah berubah.

## Level 4 — `MemoryStore`: `Add` & `Get`

```go
func NewMemoryStore() *MemoryStore
func (m *MemoryStore) Add(t Task) (Task, error)
func (m *MemoryStore) Get(id int) (Task, error)
```

Isi struct `MemoryStore` sesuai kebutuhan kalian (mis. slice `[]Task` + penghitung ID berikutnya).

- `Add` menyimpan tugas dan mengembalikannya **dengan ID baru yang unik dan bukan 0**. ID diberikan oleh store: ID yang dibawa `Task` masukan **diabaikan** (test mengirim `ID: 9999` dan mengharapkan ID lain). Judul kosong/spasi → `ErrInputKosong`.
- `Get` mengembalikan tugas dengan ID itu; kalau tidak ada → `ErrTugasTidakDitemukan`.
- `Get` mengembalikan **salinan**. Mengubah hasilnya (mis. `got.MarkDone()`) tidak boleh mengubah isi store. Kenapa ini terjadi otomatis di Go, dan apa akibatnya di API HTTP nanti? — dibahas di README (Level 10).

## Level 5 — `MemoryStore`: `List`

```go
func (m *MemoryStore) List() []Task
```

Kembalikan seluruh tugas **sesuai urutan `Add`**. Store kosong harus mengembalikan slice **kosong non-nil** (`[]Task{}`, bukan `nil`) — di Pertemuan 6 hasil ini jadi JSON, dan `nil` akan tampil `null`, bukan `[]`. Slice yang dikembalikan juga harus **salinan**: mengubah isinya, atau `append`/memotongnya, tidak boleh mengubah isi store.

## Level 6 — `MemoryStore`: `Delete`

```go
func (m *MemoryStore) Delete(id int) error
```

Hapus tugas dengan ID itu; urutan tugas yang tersisa tetap. ID tidak ada → `ErrTugasTidakDitemukan` (termasuk menghapus dua kali). **ID tidak boleh didaur ulang**: setelah tugas dihapus, `Add` berikutnya tidak boleh mendapat ID yang pernah dipakai tugas manapun sebelumnya. Coba pikirkan kenapa menghitung ID dari `len(daftar)+1` adalah bug — test ini akan menjebaknya kalau kalian memakainya.

## Level 7 — Generics: `Filter` & `Map`

```go
func Filter[T any](xs []T, pred func(T) bool) []T
func Map[T, U any](xs []T, f func(T) U) []U
```

`Filter` mengembalikan elemen yang lolos `pred` (urutan tetap). `Map` mengubah tiap elemen dengan `f` menjadi slice bertipe `U`. Aturannya: hasil tidak pernah `nil` (tanpa hasil → slice kosong non-nil), dan slice masukan **tidak boleh** dimodifikasi/dipinjam memorinya (test mengubah hasil `Filter` lalu mengecek masukan tetap utuh).

## Level 8 — Generics: `Contains` & `Max`

```go
func Contains[T comparable](xs []T, v T) bool
func Max[T cmp.Ordered](xs []T) (T, error)
```

`Contains` melaporkan apakah `v` ada di `xs`. `Max` mengembalikan elemen terbesar; slice kosong → nilai nol + `ErrDaftarKosong`. Constraint `cmp.Ordered` sudah ada di signature starter dan berlaku untuk `int`, `float64`, `string`, dan tipe turunan seperti `type Skor int` (test memakai semuanya, termasuk daftar berisi bilangan **negatif semua** — jangan memulai pencarian maksimum dari nilai `0`).

## Level 9 — Embedding `Timestamps`

`Task` sudah meng-embed `Timestamps` (composition) di starter. Kerjakan bagian logikanya:

```go
func (ts *Timestamps) Touch()
```

- `Touch` menyetel `UpdatedAt` ke `time.Now()` dan tidak menyentuh `CreatedAt`. Karena `Timestamps` di-embed, `tugas.Touch()` bisa dipanggil langsung dari `Task` (method terpromosi) — test memeriksa bahwa `Timestamps` masih anonymous embedded.
- `NewTask` (Level 1) sekarang juga mengisi `CreatedAt` **dan** `UpdatedAt` dengan waktu yang sama persis (satu kali `time.Now()`, disimpan di variabel).
- `MarkDone` dan `Rename` yang **berhasil** memanggil `Touch()`. `Rename` yang gagal (`ErrInputKosong`) **tidak** boleh memperbarui `UpdatedAt`.

## Level 10 — `Stringer` & Analisis (README)

```go
func (t Task) String() string
func Gabung[T fmt.Stringer](xs []T, sep string) string
```

`String()` mengembalikan `"[ ] 1. Belajar Go"` untuk tugas belum selesai dan `"[x] 2. Push ke GitHub"` untuk yang sudah selesai (`[<spasi atau x>] <ID>. <Judul>`). Karena **value receiver**, `fmt.Println(tugas)` maupun `fmt.Println(&tugas)` sama-sama memakai `String()` — test memeriksa keduanya, plus `%v` dan `%s`. `Gabung` menyambung hasil `String()` tiap elemen dengan pemisah `sep` (slice kosong → string kosong).

Lalu isi section `## Analisis Pertemuan 4` di `README.md` (di root repo kalian) dengan penjelasan **dengan kata-kata sendiri, minimal ±200 karakter**, yang menjawab dua pertanyaan:

1. Kenapa `var s TaskStore = MemoryStore{}` gagal dikompilasi sementara `var s TaskStore = NewMemoryStore()` berhasil? Jelaskan dengan istilah *pointer receiver* dan *method set*, bukan sekadar "compiler minta pakai &".
2. `Get` mengembalikan `Task` (salinan), bukan `*Task`. Apa akibatnya bagi kode yang memanggil `Get` lalu mengubah hasilnya — dan apa untung/ruginya dibanding mengembalikan pointer ke data di dalam store? Kaitkan dengan alasan kenapa kita memakai `interface`, bukan struct konkret.

**Dicek otomatis:** section terisi (bukan placeholder, ≥ 200 karakter), menyinggung *pointer/receiver/method set*, *salinan/copy*, dan kata *interface*. Ini hanya memastikan kalian benar-benar menjawab — kualitas penalarannya tetap akan dibaca dosen.
