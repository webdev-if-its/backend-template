// File ini disediakan dosen untuk mengecek progres level secara otomatis.
// JANGAN DIUBAH — perubahan pada file ini tidak akan dipakai saat penilaian
// (dosen menimpa ulang file ini sebelum menjalankan grading).
package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

const placeholder = "(tulis di sini)"

var headingRe = regexp.MustCompile(`(?im)^##\s+(.+?)\s*$`)

// aman menjalankan fn dan mengubah panic (mis. dari starter yang belum
// diimplementasikan) jadi kegagalan test yang rapi, bukan crash seluruh
// binary test.
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

// section mengembalikan isi di bawah heading "## <heading>" sampai heading
// berikutnya (atau akhir file), atau string kosong kalau heading tidak ada.
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

// Skor dipakai untuk memastikan Max bekerja pada tipe turunan (~int).
type Skor int

func TestLevels(t *testing.T) {
	t.Run("Level 1 - NewTask", func(t *testing.T) {
		var tk Task
		var err error
		aman(t, func() { tk, err = NewTask("  Belajar struct  ") })
		if err != nil {
			t.Fatalf("NewTask dengan judul valid seharusnya tidak error, dapat: %v", err)
		}
		if tk.Judul != "Belajar struct" {
			t.Errorf("Judul = %q, ingin %q (spasi di pinggir harus dibuang)", tk.Judul, "Belajar struct")
		}
		if tk.Selesai {
			t.Error("tugas baru seharusnya belum selesai")
		}
		// (Pengisian CreatedAt/UpdatedAt adalah bagian Level 9 -- tidak dicek di sini.)

		for _, kosong := range []string{"", "   ", "\t\n"} {
			var err error
			aman(t, func() { _, err = NewTask(kosong) })
			if !errors.Is(err, ErrInputKosong) {
				t.Errorf("NewTask(%q) harus mengembalikan ErrInputKosong, dapat: %v", kosong, err)
			}
		}
	})

	t.Run("Level 2 - MarkDone (pointer receiver)", func(t *testing.T) {
		tk := Task{ID: 1, Judul: "A"}
		aman(t, func() { tk.MarkDone() })
		if !tk.Selesai {
			t.Error("setelah MarkDone(), Selesai seharusnya true")
		}

		// Elemen slice bisa diubah langsung lewat method ber-pointer receiver.
		tugas := []Task{{ID: 1, Judul: "A"}, {ID: 2, Judul: "B"}}
		aman(t, func() { tugas[1].MarkDone() })
		if tugas[0].Selesai || !tugas[1].Selesai {
			t.Errorf("MarkDone pada tugas[1] hanya boleh mengubah tugas[1]: Selesai = [%v %v]", tugas[0].Selesai, tugas[1].Selesai)
		}

		// Memanggil MarkDone dua kali tidak boleh error/panic dan tetap selesai.
		aman(t, func() { tk.MarkDone() })
		if !tk.Selesai {
			t.Error("MarkDone dua kali harus tetap menghasilkan Selesai = true")
		}

		// Method ber-pointer receiver harus ada di *Task, bukan Task.
		if _, ok := reflect.TypeOf(Task{}).MethodByName("MarkDone"); ok {
			t.Error("MarkDone harus memakai pointer receiver (func (t *Task)), bukan value receiver")
		}
	})

	t.Run("Level 3 - Rename", func(t *testing.T) {
		tk := Task{ID: 1, Judul: "Lama"}
		var err error
		aman(t, func() { err = tk.Rename("  Baru  ") })
		if err != nil {
			t.Fatalf("Rename dengan judul valid seharusnya tidak error, dapat: %v", err)
		}
		if tk.Judul != "Baru" {
			t.Errorf("Judul = %q, ingin %q", tk.Judul, "Baru")
		}

		for _, kosong := range []string{"", "   "} {
			aman(t, func() { err = tk.Rename(kosong) })
			if !errors.Is(err, ErrInputKosong) {
				t.Errorf("Rename(%q) harus mengembalikan ErrInputKosong, dapat: %v", kosong, err)
			}
			if tk.Judul != "Baru" {
				t.Errorf("Judul tidak boleh berubah kalau Rename gagal, dapat %q", tk.Judul)
			}
		}
	})

	t.Run("Level 4 - MemoryStore: Add & Get", func(t *testing.T) {
		var s TaskStore
		aman(t, func() { s = NewMemoryStore() })

		var a, b, c Task
		var err error
		aman(t, func() { a, err = s.Add(Task{Judul: "A"}) })
		if err != nil {
			t.Fatalf("Add seharusnya tidak error, dapat: %v", err)
		}
		aman(t, func() { b, err = s.Add(Task{Judul: "B"}) })
		if err != nil {
			t.Fatalf("Add kedua seharusnya tidak error, dapat: %v", err)
		}
		if a.ID == 0 || b.ID == 0 || a.ID == b.ID {
			t.Errorf("Add harus memberi ID unik yang bukan 0, dapat a=%d b=%d", a.ID, b.ID)
		}
		// ID diberikan oleh store -- ID dari pemanggil diabaikan.
		aman(t, func() { c, err = s.Add(Task{ID: 9999, Judul: "C"}) })
		if err != nil {
			t.Fatalf("Add ketiga seharusnya tidak error, dapat: %v", err)
		}
		if c.ID == 9999 {
			t.Error("store yang memberi ID: ID yang dibawa Task masukan (9999) harus diabaikan")
		}

		var got Task
		aman(t, func() { got, err = s.Get(b.ID) })
		if err != nil || got.Judul != "B" {
			t.Fatalf("Get(%d) = judul %q, err %v; ingin judul \"B\" tanpa error", b.ID, got.Judul, err)
		}

		aman(t, func() { _, err = s.Get(424242) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Get id yang tidak ada harus ErrTugasTidakDitemukan, dapat: %v", err)
		}

		aman(t, func() { _, err = s.Add(Task{Judul: "   "}) })
		if !errors.Is(err, ErrInputKosong) {
			t.Errorf("Add dengan judul kosong harus ErrInputKosong, dapat: %v", err)
		}

		// Get mengembalikan SALINAN: mengubah hasilnya tidak boleh mengubah isi store.
		aman(t, func() { got, _ = s.Get(a.ID) })
		got.Judul = "DIUBAH DI LUAR"
		got.MarkDone()
		var lagi Task
		aman(t, func() { lagi, _ = s.Get(a.ID) })
		if lagi.Judul != "A" || lagi.Selesai {
			t.Errorf("mengubah hasil Get tidak boleh mengubah isi store: judul %q, selesai %v", lagi.Judul, lagi.Selesai)
		}
	})

	t.Run("Level 5 - MemoryStore: List", func(t *testing.T) {
		var s TaskStore
		aman(t, func() { s = NewMemoryStore() })

		var kosong []Task
		aman(t, func() { kosong = s.List() })
		if kosong == nil {
			t.Error("List pada store kosong harus mengembalikan slice kosong non-nil (nanti dipakai jadi JSON []), bukan nil")
		}
		if len(kosong) != 0 {
			t.Errorf("List pada store kosong harus len 0, dapat %d", len(kosong))
		}

		for _, j := range []string{"A", "B", "C"} {
			aman(t, func() { s.Add(Task{Judul: j}) })
		}
		var daftar []Task
		aman(t, func() { daftar = s.List() })
		if len(daftar) != 3 {
			t.Fatalf("List seharusnya 3 tugas, dapat %d", len(daftar))
		}
		for i, j := range []string{"A", "B", "C"} {
			if daftar[i].Judul != j {
				t.Errorf("urutan List harus sesuai urutan Add: daftar[%d].Judul = %q, ingin %q", i, daftar[i].Judul, j)
			}
		}

		// Mengubah slice hasil List tidak boleh mengubah isi store.
		daftar[0].Judul = "DIUBAH DI LUAR"
		daftar = append(daftar[:1], daftar[2:]...)
		var lagi []Task
		aman(t, func() { lagi = s.List() })
		if len(lagi) != 3 || lagi[0].Judul != "A" || lagi[1].Judul != "B" {
			t.Errorf("List harus mengembalikan salinan; isi store berubah setelah slice hasil dimodifikasi (len=%d)", len(lagi))
		}
	})

	t.Run("Level 6 - MemoryStore: Delete", func(t *testing.T) {
		var s TaskStore
		aman(t, func() { s = NewMemoryStore() })

		var a, b, c Task
		aman(t, func() {
			a, _ = s.Add(Task{Judul: "A"})
			b, _ = s.Add(Task{Judul: "B"})
			c, _ = s.Add(Task{Judul: "C"})
		})

		var err error
		aman(t, func() { err = s.Delete(b.ID) })
		if err != nil {
			t.Fatalf("Delete(%d) seharusnya tidak error, dapat: %v", b.ID, err)
		}
		aman(t, func() { _, err = s.Get(b.ID) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("setelah Delete, Get id itu harus ErrTugasTidakDitemukan, dapat: %v", err)
		}
		var daftar []Task
		aman(t, func() { daftar = s.List() })
		if len(daftar) != 2 || daftar[0].Judul != "A" || daftar[1].Judul != "C" {
			t.Errorf("setelah Delete tengah, List harus tersisa A lalu C (urutan tetap), dapat %d tugas", len(daftar))
		}

		aman(t, func() { err = s.Delete(b.ID) })
		if !errors.Is(err, ErrTugasTidakDitemukan) {
			t.Errorf("Delete id yang sudah tidak ada harus ErrTugasTidakDitemukan, dapat: %v", err)
		}

		// ID tidak boleh dipakai ulang setelah dihapus (jangan hitung ID dari len(daftar)+1).
		aman(t, func() { err = s.Delete(c.ID) })
		if err != nil {
			t.Fatalf("Delete(%d) seharusnya tidak error, dapat: %v", c.ID, err)
		}
		var d Task
		aman(t, func() { d, _ = s.Add(Task{Judul: "D"}) })
		for _, lama := range []int{a.ID, b.ID, c.ID} {
			if d.ID == lama {
				t.Errorf("ID %d dipakai ulang untuk tugas baru; ID tidak boleh didaur ulang setelah dihapus", lama)
			}
		}
	})

	t.Run("Level 7 - Generics: Filter & Map", func(t *testing.T) {
		angka := []int{1, 2, 3, 4, 5, 6}
		var genap []int
		aman(t, func() { genap = Filter(angka, func(n int) bool { return n%2 == 0 }) })
		if !reflect.DeepEqual(genap, []int{2, 4, 6}) {
			t.Errorf("Filter genap = %v, ingin [2 4 6]", genap)
		}
		genap[0] = 999
		if angka[1] != 2 {
			t.Error("Filter tidak boleh mengubah/meminjam memori slice masukan")
		}

		var tidakAda []int
		aman(t, func() { tidakAda = Filter(angka, func(n int) bool { return n > 100 }) })
		if tidakAda == nil || len(tidakAda) != 0 {
			t.Errorf("Filter tanpa hasil harus mengembalikan slice kosong non-nil, dapat %#v", tidakAda)
		}

		tugas := []Task{{ID: 1, Judul: "A", Selesai: true}, {ID: 2, Judul: "B"}, {ID: 3, Judul: "C", Selesai: true}}
		var selesai []Task
		aman(t, func() { selesai = Filter(tugas, func(tk Task) bool { return tk.Selesai }) })
		if len(selesai) != 2 || selesai[0].ID != 1 || selesai[1].ID != 3 {
			t.Errorf("Filter tugas selesai salah, dapat %d tugas", len(selesai))
		}

		var judul []string
		aman(t, func() { judul = Map(tugas, func(tk Task) string { return tk.Judul }) })
		if !reflect.DeepEqual(judul, []string{"A", "B", "C"}) {
			t.Errorf("Map judul = %v, ingin [A B C]", judul)
		}
		var kuadrat []int
		aman(t, func() { kuadrat = Map([]int{1, 2, 3}, func(n int) int { return n * n }) })
		if !reflect.DeepEqual(kuadrat, []int{1, 4, 9}) {
			t.Errorf("Map kuadrat = %v, ingin [1 4 9]", kuadrat)
		}
		var petaKosong []string
		aman(t, func() { petaKosong = Map([]int(nil), func(n int) string { return "x" }) })
		if petaKosong == nil || len(petaKosong) != 0 {
			t.Errorf("Map pada masukan kosong harus mengembalikan slice kosong non-nil, dapat %#v", petaKosong)
		}
	})

	t.Run("Level 8 - Generics: Contains & Max", func(t *testing.T) {
		aman(t, func() {
			if !Contains([]string{"a", "b"}, "b") || Contains([]string{"a", "b"}, "z") {
				t.Error("Contains string salah")
			}
			if !Contains([]int{1, 2, 3}, 3) || Contains([]int(nil), 1) {
				t.Error("Contains int salah")
			}
			if !Contains([]Skor{1, 2}, Skor(2)) {
				t.Error("Contains pada tipe turunan (Skor) salah")
			}
		})

		aman(t, func() {
			if m, err := Max([]int{3, 9, 2}); err != nil || m != 9 {
				t.Errorf("Max int = %v, err %v; ingin 9", m, err)
			}
			if m, err := Max([]float64{1.5, -2, 0.25}); err != nil || m != 1.5 {
				t.Errorf("Max float64 = %v, err %v; ingin 1.5", m, err)
			}
			if m, err := Max([]string{"apel", "mangga", "jeruk"}); err != nil || m != "mangga" {
				t.Errorf("Max string = %q, err %v; ingin \"mangga\"", m, err)
			}
			if m, err := Max([]Skor{70, 95, 80}); err != nil || m != Skor(95) {
				t.Errorf("Max Skor = %v, err %v; ingin 95", m, err)
			}
			if m, err := Max([]int{-5, -2, -9}); err != nil || m != -2 {
				t.Errorf("Max semua-negatif = %v, err %v; ingin -2 (jangan mulai dari 0)", m, err)
			}
			_, err := Max([]int{})
			if !errors.Is(err, ErrDaftarKosong) {
				t.Errorf("Max pada slice kosong harus ErrDaftarKosong, dapat: %v", err)
			}
		})
	})

	t.Run("Level 9 - Embedding Timestamps", func(t *testing.T) {
		var tk Task
		var err error
		aman(t, func() { tk, err = NewTask("Belajar embedding") })
		if err != nil {
			t.Fatalf("NewTask seharusnya tidak error, dapat: %v", err)
		}
		if tk.CreatedAt.IsZero() || tk.UpdatedAt.IsZero() {
			t.Fatal("NewTask harus mengisi CreatedAt dan UpdatedAt")
		}
		if !tk.CreatedAt.Equal(tk.UpdatedAt) {
			t.Error("pada tugas yang baru dibuat, CreatedAt dan UpdatedAt harus sama")
		}
		if time.Since(tk.CreatedAt) > time.Minute {
			t.Error("CreatedAt harus berisi waktu sekarang (time.Now())")
		}

		// Timestamps harus benar-benar di-embed (bukan field bernama biasa), sehingga Touch() terpromosi.
		f, ok := reflect.TypeOf(Task{}).FieldByName("Timestamps")
		if !ok || !f.Anonymous {
			t.Error("Task harus meng-embed Timestamps (tanpa nama field), bukan field bernama biasa")
		}

		tk.UpdatedAt = time.Time{}
		aman(t, func() { tk.Touch() })
		if tk.UpdatedAt.IsZero() {
			t.Error("Touch() harus mengisi UpdatedAt dengan waktu sekarang")
		}
		if tk.CreatedAt.IsZero() {
			t.Error("Touch() tidak boleh menyentuh CreatedAt")
		}

		tk.UpdatedAt = time.Time{}
		aman(t, func() { tk.MarkDone() })
		if tk.UpdatedAt.IsZero() {
			t.Error("MarkDone() harus memperbarui UpdatedAt (panggil Touch)")
		}
		tk.UpdatedAt = time.Time{}
		aman(t, func() { err = tk.Rename("Judul baru") })
		if err != nil || tk.UpdatedAt.IsZero() {
			t.Errorf("Rename() yang berhasil harus memperbarui UpdatedAt (err=%v)", err)
		}
		tk.UpdatedAt = time.Time{}
		aman(t, func() { _ = tk.Rename("  ") })
		if !tk.UpdatedAt.IsZero() {
			t.Error("Rename() yang gagal tidak boleh memperbarui UpdatedAt")
		}
	})

	t.Run("Level 10 - Stringer & analisis README", func(t *testing.T) {
		belum := Task{ID: 1, Judul: "Belajar Go"}
		sudah := Task{ID: 2, Judul: "Push ke GitHub", Selesai: true}

		aman(t, func() {
			if got := fmt.Sprint(belum); got != "[ ] 1. Belajar Go" {
				t.Errorf("String() tugas belum selesai = %q, ingin %q", got, "[ ] 1. Belajar Go")
			}
			if got := fmt.Sprint(sudah); got != "[x] 2. Push ke GitHub" {
				t.Errorf("String() tugas selesai = %q, ingin %q", got, "[x] 2. Push ke GitHub")
			}
			// Value receiver: pointer ke Task juga harus tercetak dengan format yang sama.
			if got := fmt.Sprint(&sudah); got != "[x] 2. Push ke GitHub" {
				t.Errorf("fmt.Sprint(&task) = %q, ingin %q -- String() harus value receiver", got, "[x] 2. Push ke GitHub")
			}
			if got := fmt.Sprintf("%v|%s", belum, belum); got != "[ ] 1. Belajar Go|[ ] 1. Belajar Go" {
				t.Errorf("%%v dan %%s harus sama-sama memakai String(), dapat %q", got)
			}
		})

		aman(t, func() {
			if got := Gabung([]Task{belum, sudah}, "; "); got != "[ ] 1. Belajar Go; [x] 2. Push ke GitHub" {
				t.Errorf("Gabung tugas = %q", got)
			}
			if got := Gabung([]Task{}, ","); got != "" {
				t.Errorf("Gabung slice kosong harus string kosong, dapat %q", got)
			}
			if got := Gabung([]fmt.Stringer{belum}, ","); got != "[ ] 1. Belajar Go" {
				t.Errorf("Gabung satu elemen = %q", got)
			}
		})

		s := section(readFile("../README.md"), "Analisis Pertemuan 4")
		if !filled(s, 200) {
			t.Fatalf("section '## Analisis Pertemuan 4' di README.md (root repo) tidak ada atau belum diisi memadai (minimal 200 karakter, saat ini %d) -- tambahkan heading persis itu kalau belum ada", len(strings.TrimSpace(s)))
		}
		if !mengandungSalahSatu(s, "pointer", "receiver", "method set") {
			t.Error("analisis harus menjelaskan soal pointer/receiver (kenapa MemoryStore tanpa & tidak memenuhi TaskStore)")
		}
		if !mengandungSalahSatu(s, "salinan", "copy", "duplikat", "disalin") {
			t.Error("analisis harus membahas Get yang mengembalikan salinan Task (bukan pointer)")
		}
		if !mengandungSalahSatu(s, "interface") {
			t.Error("analisis harus menyebut kata 'interface'")
		}
	})
}
