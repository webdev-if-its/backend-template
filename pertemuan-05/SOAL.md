# Pertemuan 5 — Concurrent File Downloader

> **Catatan (opsional/bonus):** Pertemuan 5 diisi Quiz 1 dan pengenalan concurrency secara ringkas, jadi tugas ini **tidak wajib**. Kerjakan kalau ingin memperdalam concurrency; level yang lolos dicatat sebagai nilai tambahan.

Tugas ini **berdiri sendiri** (bukan bagian dari Task Manager): memaksakan concurrency ke CRUD sederhana akan terasa dibuat-buat, jadi kita berlatih dengan kasus yang memang cocok — mengunduh banyak file sekaligus. Mahasiswa yang ingin tantangan boleh mencoba menerapkan pola yang sama di Task Manager sebagai latihan tambahan (tidak dinilai).

**Unduhannya disimulasikan** dengan `time.Sleep` — tidak ada jaringan sungguhan, supaya hasil test deterministik dan tidak bergantung pada internet. Semua kode ada di `pertemuan-05/main.go` (starter berisi kerangka lengkap dengan `panic("belum diimplementasikan")`; jangan ubah signature-nya). Gunakan `sync.WaitGroup` dengan `Add`/`Done` klasik, **jangan** `wg.Go` (baru ada di Go 1.25 — Go di laptop kalian bisa lebih lama).

**Cara test ini mengecek konkurensi** (penting kalian tahu, supaya tidak heran kenapa gagal): test **tidak pernah** mengandalkan urutan cetak atau ketepatan waktu. Ia menyuntikkan "pengunduh palsu" yang baru mau lanjut kalau sejumlah unduhan *sedang berjalan bersamaan* — kode yang diam-diam berjalan satu per satu tidak akan pernah membuka "gerbang" itu dan level-nya gagal. Setiap pemanggilan diberi batas waktu, jadi kode yang deadlock hanya menggagalkan level itu (tidak menggantung `go test`). Test konkurensi bisa flaky kalau kodenya salah — jalankan `go test -count=5` sebelum push untuk memastikan konsisten. (Flag `-race` sengaja tidak dipakai: butuh cgo yang tidak selalu tersedia di Windows.)

Ada satu section README yang dicek di Level 10 (`## Analisis Pertemuan 5`) — **tambahkan sendiri** heading persis itu di `README.md` root repo kalian (repo dibuat dari template di awal semester, jadi heading-nya belum ada).

---

## Tipe yang sudah disediakan

```go
type File struct{ Nama string; UkuranKB int; Durasi time.Duration }
type Hasil struct{ Nama string; UkuranKB int; Err error }
type Pengunduh func(File) Hasil // "cara mengunduh satu file" -- disuntikkan lewat parameter
```

## Level 1 — `Jalankan`

```go
func Jalankan(fn func())
```

Jalankan `fn` di **goroutine baru** dan **langsung kembali** — jangan menunggu `fn` selesai. Test memberi `fn` yang sengaja menunggu sinyal dari test; kalau `Jalankan` memanggil `fn()` biasa atau menunggunya, ia tidak akan pernah kembali dan level gagal.

## Level 2 — `JalankanN`

```go
func JalankanN(n int, fn func(i int))
```

Jalankan `fn(0)`, `fn(1)`, …, `fn(n-1)`, masing-masing di goroutine sendiri, lalu **langsung kembali**. Tiap `i` harus tepat satu kali dipanggil — test mengecek tidak ada `i` yang terlewat atau ganda. Ini jebakan klasik: goroutine yang membaca variabel loop yang dipakai bersama. Cara paling aman dan tidak bergantung versi Go: oper `i` sebagai **parameter** ke fungsi goroutine.

## Level 3 — `Unduh` & `UnduhBerurutan`

```go
func Unduh(f File) Hasil
func UnduhBerurutan(files []File) []Hasil
```

`Unduh` mensimulasikan satu unduhan: kalau `f.UkuranKB <= 0`, kembalikan `Hasil` dengan `Err = ErrUkuranTidakValid` **tanpa menunggu**; kalau valid, `time.Sleep(f.Durasi)` lalu kembalikan `Hasil{Nama, UkuranKB, nil}`. `UnduhBerurutan` mengunduh file **satu per satu**, hasil sesuai urutan `files` — ini pembanding: dengan 3 file × 20ms ia harus makan ≥ 60ms. Nanti kalian bandingkan dengan versi konkuren.

## Level 4 — `UnduhSemua` dengan `WaitGroup`

```go
func UnduhSemua(files []File, unduh Pengunduh) []Hasil
```

Unduh **semua file bersamaan** (satu goroutine per file), **tunggu semuanya selesai** dengan `sync.WaitGroup`, lalu kembalikan hasilnya. `wg.Add` dipanggil **sebelum** `go`, dan `wg.Done()` lewat `defer`. Test memakai pengunduh palsu yang menahan diri sampai 6 panggilan hadir bersamaan — kalau puncak konkurensi yang terukur bukan 6, level gagal. Panggil `unduh(f)` (parameter), **bukan** `Unduh` langsung.

## Level 5 — `UnduhSemua`: urutan hasil & masukan kosong

Fungsi yang sama. Hasil harus **sesuai urutan `files`**, bukan urutan selesai: `hasil[i]` adalah untuk `files[i]` walau file pertama selesai paling akhir (test membuat file awal paling lambat). Masukan `nil`/kosong harus mengembalikan slice kosong **non-nil** tanpa macet. Satu file gagal (mis. `UkuranKB: 0`) hanya mengisi `Err` miliknya sendiri dan tidak mengganggu yang lain.

Petunjuk penalaran: bagaimana caranya banyak goroutine mengisi satu slice hasil **tanpa** mutex dan tetap benar? — jawabannya ada di pertanyaan README (Level 10).

## Level 6 — `UnduhSemua`: panic di dalam goroutine

Fungsi yang sama. Kalau `unduh(f)` **panic** (mis. bug di kode pengunduh), `UnduhSemua` tidak boleh ikut mati: ubah panic itu menjadi `Hasil.Err` (berisi pesan panic asli, mis. `fmt.Errorf("unduhan panic: %v", r)`) untuk file tersebut, sementara file lain tetap selesai normal dan `wg.Done()` tetap dipanggil.

Kenapa ini level tersendiri: `recover()` hanya menangkap panic **di goroutine yang sama** — `defer`+`recover` di fungsi `UnduhSemua` tidak menolong panic yang terjadi di goroutine anak. Panic yang lolos dari sebuah goroutine mematikan **seluruh program**. Karena itu test level ini dijalankan di **proses anak terpisah** (supaya kalau kode kalian belum menanganinya, level lain tidak ikut mati) — keluaran proses itu ditampilkan di pesan gagal.

## Level 7 — `UnduhKeChannel`

```go
func UnduhKeChannel(files []File, unduh Pengunduh) <-chan Hasil
```

Mulai semua unduhan bersamaan dan **langsung kembalikan** sebuah channel; tiap hasil dikirim ke channel begitu selesai (urutan sesuai waktu selesai — test tidak mengecek urutan). Channel harus **ditutup setelah semua hasil terkirim**, oleh pengirim (bukan penerima), sehingga `for h := range ch` berhenti sendiri. Tips: satu goroutine tambahan yang `wg.Wait()` lalu `close(ch)`. `files` kosong → channel yang langsung tertutup. Pertimbangkan: apa yang terjadi kalau channel-nya tak berpenyangga dan fungsi ini sendiri yang `wg.Wait()` sebelum kembali?

## Level 8 — `UnduhBatas` (buffered channel sebagai semaphore)

```go
func UnduhBatas(files []File, unduh Pengunduh, maks int) ([]Hasil, error)
```

Seperti `UnduhSemua` (hasil sesuai urutan `files`, tunggu semua selesai), tapi **paling banyak `maks` unduhan berjalan pada saat yang sama** — mengunduh 1000 file sekaligus adalah cara cepat membuat server tujuan (atau laptop kalian) kewalahan. Pola standarnya: buffered channel berkapasitas `maks` sebagai semaphore (kirim untuk "ambil slot", terima untuk "kembalikan slot"). `maks < 1` → `nil, ErrBatasTidakValid` tanpa memulai unduhan apa pun. Test memeriksa puncak konkurensi **tidak melebihi** `maks` **dan** benar-benar mencapainya (jangan membatasi lebih ketat dari yang diminta); untuk `maks` lebih besar dari jumlah file, semuanya boleh berjalan bersamaan.

## Level 9 — `GabungChannel` (fan-in)

```go
func GabungChannel(chs ...<-chan Hasil) <-chan Hasil
```

Gabungkan banyak channel masukan menjadi **satu** channel keluaran (urutan antar channel bebas), yang **ditutup hanya setelah semua channel masukan tertutup**. Tanpa argumen → channel yang langsung tertutup. Test memakai tiga produsen: satu mengirim 3 hasil, satu langsung kosong, satu baru mulai mengirim 5 hasil setelah jeda — menutup channel gabungan terlalu cepat (atau tidak pernah) membuat level gagal. Ini kombinasi tiga hal yang kalian pelajari: goroutine per channel masukan, `WaitGroup` untuk tahu kapan *semua* selesai, dan `close` oleh satu pihak saja.

## Level 10 — `Progres` (Mutex), `Ringkasan` & Analisis (README)

```go
type Ringkasan struct{ Total, Selesai, Gagal, TotalKB int }
func (r Ringkasan) String() string
func NewProgres(total int) *Progres
func (p *Progres) Catat(h Hasil)
func (p *Progres) Snapshot() Ringkasan
```

`Progres` adalah penghitung yang **dipakai banyak goroutine sekaligus**. `NewProgres(total)` mengisi `Total`. `Catat(h)`: kalau `h.Err != nil` → `Gagal++`; kalau tidak → `Selesai++` dan `TotalKB += h.UkuranKB` (unduhan gagal tidak menambah `TotalKB`). `Snapshot()` mengembalikan salinan `Ringkasan` saat ini. Lindungi semuanya dengan `sync.Mutex` (field `mu` sudah ada di starter) — test memanggil `Catat` dari 100 goroutine × 500 kali dan mengharapkan hitungan **persis**; tanpa mutex ada update yang hilang.

`Ringkasan.String()` mengembalikan persis: `"<Selesai> dari <Total> berhasil, <Gagal> gagal, total <TotalKB> KB"`, contoh `"2 dari 3 berhasil, 1 gagal, total 150 KB"`.

Lalu isi `## Analisis Pertemuan 5` di `README.md` (root repo) — dengan kata-kata sendiri, **minimal ±200 karakter**, jawab dua pertanyaan:

1. Kenapa `hasil[i] = ...` dari banyak goroutine (indeks berbeda) aman **tanpa** Mutex di `UnduhSemua`, sedangkan `selesai++` pada `Progres` dari banyak goroutine **tidak** aman? Jelaskan apa itu *race condition* dan kenapa `selesai++` bukan satu langkah.
2. Apa yang salah kalau `wg.Add(1)` ditulis **di dalam** goroutine, bukan sebelum `go`? Kaitkan dengan kapan `wg.Wait()` bisa lewat.

**Dicek otomatis:** section terisi (≥ 200 karakter, bukan placeholder), menyebut *race/balapan*, *mutex/lock/kunci*, dan *indeks/index/elemen*. Ini hanya memastikan kalian benar-benar menjawab — kualitas penalarannya tetap dibaca dosen.
