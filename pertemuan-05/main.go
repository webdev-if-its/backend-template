package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// TODO: lihat SOAL.md untuk kontrak lengkap tiap fungsi di bawah.
// Ganti setiap "panic" dengan implementasi yang benar. Tambahkan import
// sendiri kalau dibutuhkan. Gunakan sync.WaitGroup dengan Add/Done klasik
// (jangan wg.Go -- baru ada di Go 1.25).

// ErrUkuranTidakValid dikembalikan (lewat Hasil.Err) untuk file berukuran <= 0.
var ErrUkuranTidakValid = errors.New("ukuran file tidak valid")

// ErrBatasTidakValid dikembalikan UnduhBatas kalau batas paralel < 1.
var ErrBatasTidakValid = errors.New("batas paralel harus minimal 1")

// File adalah satu file yang akan "diunduh". Unduhan di tugas ini
// DISIMULASIKAN dengan time.Sleep -- tidak ada jaringan sungguhan.
type File struct {
	Nama     string
	UkuranKB int
	Durasi   time.Duration // lamanya simulasi unduhan
}

// Hasil adalah hasil satu unduhan.
type Hasil struct {
	Nama     string
	UkuranKB int
	Err      error
}

// Pengunduh adalah fungsi yang mengunduh satu file. Fungsi-fungsi di bawah
// menerimanya sebagai parameter (bukan memanggil Unduh langsung) supaya test
// bisa menyuntikkan unduhan palsu dan mengamati konkurensinya.
type Pengunduh func(File) Hasil

// Jalankan menjalankan fn di goroutine baru dan LANGSUNG kembali. (Level 1)
func Jalankan(fn func()) {
	panic("belum diimplementasikan")
}

// JalankanN menjalankan fn(0) ... fn(n-1), masing-masing di goroutine sendiri,
// dan LANGSUNG kembali (tidak menunggu). (Level 2)
func JalankanN(n int, fn func(i int)) {
	panic("belum diimplementasikan")
}

// Unduh mensimulasikan satu unduhan. (Level 3)
func Unduh(f File) Hasil {
	panic("belum diimplementasikan")
}

// UnduhBerurutan mengunduh file satu per satu -- pembanding. (Level 3)
func UnduhBerurutan(files []File) []Hasil {
	panic("belum diimplementasikan")
}

// UnduhSemua mengunduh semua file BERSAMAAN dengan sync.WaitGroup dan
// mengembalikan hasilnya SESUAI URUTAN files. (Level 4-6)
func UnduhSemua(files []File, unduh Pengunduh) []Hasil {
	panic("belum diimplementasikan")
}

// UnduhKeChannel mengunduh semua file bersamaan dan mengirim tiap hasil ke
// channel begitu selesai; channel ditutup setelah semuanya selesai. (Level 7)
func UnduhKeChannel(files []File, unduh Pengunduh) <-chan Hasil {
	panic("belum diimplementasikan")
}

// UnduhBatas seperti UnduhSemua tapi paling banyak `maks` unduhan berjalan
// pada saat yang sama. (Level 8)
func UnduhBatas(files []File, unduh Pengunduh, maks int) ([]Hasil, error) {
	panic("belum diimplementasikan")
}

// GabungChannel menggabungkan banyak channel menjadi satu (fan-in); channel
// hasil ditutup setelah SEMUA channel masukan tertutup. (Level 9)
func GabungChannel(chs ...<-chan Hasil) <-chan Hasil {
	panic("belum diimplementasikan")
}

// Ringkasan adalah potret progres unduhan. (Level 10)
type Ringkasan struct {
	Total   int
	Selesai int
	Gagal   int
	TotalKB int
}

// String memformat ringkasan untuk laporan. (Level 10)
func (r Ringkasan) String() string {
	panic("belum diimplementasikan")
}

// Progres mencatat progres unduhan dan AMAN dipakai banyak goroutine
// sekaligus. (Level 10)
type Progres struct {
	mu sync.Mutex
	// TODO: tambahkan field penghitung yang kalian butuhkan
}

// NewProgres membuat Progres untuk `total` unduhan. (Level 10)
func NewProgres(total int) *Progres {
	panic("belum diimplementasikan")
}

// Catat mencatat satu hasil unduhan. (Level 10)
func (p *Progres) Catat(h Hasil) {
	panic("belum diimplementasikan")
}

// Snapshot mengembalikan potret progres saat ini. (Level 10)
func (p *Progres) Snapshot() Ringkasan {
	panic("belum diimplementasikan")
}

func main() {
	fmt.Println("Concurrent File Downloader - pertemuan 5")

	files := []File{
		{"a.zip", 120, 300 * time.Millisecond},
		{"b.pdf", 80, 200 * time.Millisecond},
		{"c.mp4", 900, 500 * time.Millisecond},
		{"rusak.bin", 0, 100 * time.Millisecond},
	}

	mulai := time.Now()
	hasil := UnduhSemua(files, Unduh)
	progres := NewProgres(len(files))
	for _, h := range hasil {
		progres.Catat(h)
	}
	fmt.Println(progres.Snapshot())
	fmt.Println("selesai dalam", time.Since(mulai).Round(10*time.Millisecond))
}
