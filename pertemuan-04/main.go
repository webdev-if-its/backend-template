package main

import (
	"cmp"
	"errors"
	"fmt"
	"time"
)

// TODO: lihat SOAL.md untuk kontrak lengkap tiap fungsi/method di bawah.
// Ganti setiap "panic" dengan implementasi yang benar. Tambahkan import
// (mis. "strings") sendiri kalau memang dibutuhkan.

// ErrTugasTidakDitemukan dikembalikan ketika ID tugas tidak ada.
var ErrTugasTidakDitemukan = errors.New("tugas tidak ditemukan")

// ErrInputKosong dikembalikan ketika judul kosong (atau hanya spasi).
var ErrInputKosong = errors.New("input tidak boleh kosong")

// ErrDaftarKosong dikembalikan oleh Max ketika slice-nya kosong.
var ErrDaftarKosong = errors.New("daftar kosong")

// Timestamps dipakai lewat embedding (composition) di Task.
type Timestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Touch menyetel UpdatedAt ke waktu sekarang. (Level 9)
func (ts *Timestamps) Touch() {
	panic("belum diimplementasikan")
}

// Task merepresentasikan satu tugas.
type Task struct {
	ID      int
	Judul   string
	Selesai bool
	Timestamps
}

// NewTask membuat Task baru dari judul. (Level 1)
func NewTask(judul string) (Task, error) {
	panic("belum diimplementasikan")
}

// MarkDone menandai tugas selesai. (Level 2)
func (t *Task) MarkDone() {
	panic("belum diimplementasikan")
}

// Rename mengganti judul tugas. (Level 3)
func (t *Task) Rename(judul string) error {
	panic("belum diimplementasikan")
}

// String membuat Task memenuhi fmt.Stringer. (Level 10)
func (t Task) String() string {
	panic("belum diimplementasikan")
}

// TaskStore adalah kontrak penyimpanan tugas. JANGAN diubah -- yang kalian
// tulis adalah implementasinya (MemoryStore).
type TaskStore interface {
	Add(t Task) (Task, error)
	Get(id int) (Task, error)
	List() []Task
	Delete(id int) error
}

// MemoryStore menyimpan tugas di memori.
type MemoryStore struct {
	// TODO: tambahkan field yang kalian butuhkan (mis. slice tugas dan penghitung ID)
}

// Pastikan *MemoryStore memenuhi TaskStore -- kalau tidak, kode gagal
// dikompilasi di sini, bukan baru ketahuan saat dijalankan.
var _ TaskStore = (*MemoryStore)(nil)

// NewMemoryStore membuat store kosong.
func NewMemoryStore() *MemoryStore {
	panic("belum diimplementasikan")
}

// Add menyimpan tugas dan memberinya ID baru. (Level 4)
func (m *MemoryStore) Add(t Task) (Task, error) {
	panic("belum diimplementasikan")
}

// Get mencari tugas menurut ID. (Level 4)
func (m *MemoryStore) Get(id int) (Task, error) {
	panic("belum diimplementasikan")
}

// List mengembalikan seluruh tugas. (Level 5)
func (m *MemoryStore) List() []Task {
	panic("belum diimplementasikan")
}

// Delete menghapus tugas menurut ID. (Level 6)
func (m *MemoryStore) Delete(id int) error {
	panic("belum diimplementasikan")
}

// Filter mengembalikan elemen xs yang lolos pred. (Level 7)
func Filter[T any](xs []T, pred func(T) bool) []T {
	panic("belum diimplementasikan")
}

// Map mengubah tiap elemen xs dengan f. (Level 7)
func Map[T, U any](xs []T, f func(T) U) []U {
	panic("belum diimplementasikan")
}

// Contains melaporkan apakah v ada di xs. (Level 8)
func Contains[T comparable](xs []T, v T) bool {
	panic("belum diimplementasikan")
}

// Max mengembalikan elemen terbesar di xs. (Level 8)
func Max[T cmp.Ordered](xs []T) (T, error) {
	panic("belum diimplementasikan")
}

// Gabung menyambung String() tiap elemen dengan pemisah sep. (Level 10)
func Gabung[T fmt.Stringer](xs []T, sep string) string {
	panic("belum diimplementasikan")
}

func main() {
	fmt.Println("Task Manager v1 - pertemuan 4")

	var store TaskStore = NewMemoryStore()
	for _, judul := range []string{"Belajar struct", "Belajar interface"} {
		t, err := NewTask(judul)
		if err != nil {
			fmt.Println("gagal membuat tugas:", err)
			continue
		}
		if _, err := store.Add(t); err != nil {
			fmt.Println("gagal menyimpan tugas:", err)
		}
	}
	for _, t := range store.List() {
		fmt.Println(t)
	}
}
