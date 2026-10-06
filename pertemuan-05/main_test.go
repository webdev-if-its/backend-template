// File ini disediakan dosen untuk mengecek progres level secara otomatis.
// JANGAN DIUBAH — perubahan pada file ini tidak akan dipakai saat penilaian
// (dosen menimpa ulang file ini sebelum menjalankan grading).
//
// Catatan desain: test konkurensi TIDAK pernah mengandalkan urutan cetak atau
// ketepatan waktu; yang dicek adalah hasil yang dikumpulkan, jumlah panggilan
// yang benar-benar berjalan bersamaan (lewat "gerbang" yang baru terbuka kalau
// cukup banyak unduhan hadir bersamaan), dan batas waktu supaya kode yang
// deadlock menggagalkan level -- bukan menggantung `go test`.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const placeholder = "(tulis di sini)"

var headingRe = regexp.MustCompile(`(?im)^##\s+(.+?)\s*$`)

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

// dalam menjalankan fn di goroutine tersendiri dengan batas waktu d. Panic
// (mis. dari starter yang belum diimplementasikan) dan deadlock/menunggu
// tanpa akhir diubah menjadi kegagalan test yang rapi. Di dalam fn pakai
// t.Errorf, JANGAN t.Fatalf (fn tidak jalan di goroutine test).
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
		t.Fatalf("melebihi batas waktu %v -- deadlock, atau fungsi menunggu tanpa akhir?", d)
	}
}

// panggil menjalankan fn di goroutine sendiri dan mengirim nilai panic-nya
// (nil kalau tidak panic) ke channel hasil begitu fn kembali.
func panggil(fn func()) <-chan any {
	ch := make(chan any, 1)
	go func() {
		defer func() { ch <- recover() }()
		fn()
	}()
	return ch
}

func buatFile(n int) []File {
	files := make([]File, n)
	for i := range files {
		files[i] = File{Nama: fmt.Sprintf("f%02d", i), UkuranKB: 10 * (i + 1), Durasi: time.Millisecond}
	}
	return files
}

// pelacak adalah Pengunduh palsu yang mencatat berapa unduhan berjalan
// bersamaan. Kalau tunggu > 0, tiap panggilan menahan diri sampai `tunggu`
// panggilan hadir bersamaan (gerbang terbuka) -- kode yang tidak benar-benar
// konkuren tidak akan pernah membuka gerbang itu (ditunggu paling lama 300ms).
type pelacak struct {
	mu     sync.Mutex
	aktif  int
	puncak int
	total  int
	tunggu int
	buka   chan struct{}
	dibuka bool
}

func barrierPelacak(tunggu int) *pelacak {
	return &pelacak{tunggu: tunggu, buka: make(chan struct{})}
}

func (p *pelacak) unduh(f File) Hasil {
	p.mu.Lock()
	p.aktif++
	p.total++
	if p.aktif > p.puncak {
		p.puncak = p.aktif
	}
	if p.tunggu > 0 && !p.dibuka && p.aktif >= p.tunggu {
		p.dibuka = true
		close(p.buka)
	}
	p.mu.Unlock()

	if p.tunggu > 0 {
		select {
		case <-p.buka:
		case <-time.After(300 * time.Millisecond):
		}
	}
	time.Sleep(3 * time.Millisecond)

	p.mu.Lock()
	p.aktif--
	p.mu.Unlock()
	return Hasil{Nama: f.Nama, UkuranKB: f.UkuranKB}
}

func (p *pelacak) hasilPuncak() (puncak, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.puncak, p.total
}

func namaSet(hasil []Hasil) []string {
	out := make([]string, len(hasil))
	for i, h := range hasil {
		out[i] = h.Nama
	}
	sort.Strings(out)
	return out
}

func namaFile(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Nama
	}
	sort.Strings(out)
	return out
}

func samaSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAnakPanic dijalankan HANYA sebagai proses anak oleh Level 6: unduhan yang
// panic di dalam goroutine yang tidak di-recover akan mematikan seluruh proses
// -- kalau itu terjadi di proses utama, level-level lain ikut mati. Jadi
// skenarionya diisolasi di proses terpisah.
func TestAnakPanic(t *testing.T) {
	if os.Getenv("P5_ANAK") != "1" {
		t.Skip("hanya dijalankan sebagai proses anak oleh Level 6")
	}
	files := []File{{Nama: "a", UkuranKB: 1}, {Nama: "rusak", UkuranKB: 1}, {Nama: "c", UkuranKB: 1}}
	unduhPanic := func(f File) Hasil {
		if f.Nama == "rusak" {
			panic("meledak")
		}
		return Hasil{Nama: f.Nama, UkuranKB: f.UkuranKB}
	}
	var hasil []Hasil
	dalam(t, 3*time.Second, func() { hasil = UnduhSemua(files, unduhPanic) })
	if len(hasil) != 3 {
		t.Fatalf("UnduhSemua harus mengembalikan 3 hasil walau satu unduhan panic, dapat %d", len(hasil))
	}
	if hasil[1].Err == nil || !strings.Contains(hasil[1].Err.Error(), "meledak") {
		t.Errorf("hasil unduhan yang panic harus memuat Err berisi pesan panic, dapat: %v", hasil[1].Err)
	}
	if hasil[0].Err != nil || hasil[2].Err != nil || hasil[0].Nama != "a" || hasil[2].Nama != "c" {
		t.Errorf("unduhan lain tidak boleh terpengaruh: %+v", hasil)
	}
}

func TestLevels(t *testing.T) {
	t.Run("Level 1 - Jalankan (goroutine, tidak menunggu)", func(t *testing.T) {
		mulai := make(chan struct{})
		lepas := make(chan struct{})
		selesai := make(chan struct{})

		kembali := panggil(func() {
			Jalankan(func() {
				close(mulai)
				<-lepas
				close(selesai)
			})
		})

		select {
		case r := <-kembali:
			if r != nil {
				t.Fatalf("Jalankan panic: %v (kemungkinan belum diimplementasikan)", r)
			}
		case <-time.After(time.Second):
			t.Fatal("Jalankan tidak kembali padahal fn masih berjalan -- fn harus dijalankan di goroutine baru (go fn()), bukan dipanggil langsung / ditunggu")
		}
		select {
		case <-mulai:
		case <-time.After(time.Second):
			t.Fatal("fn tidak pernah dijalankan")
		}
		close(lepas)
		select {
		case <-selesai:
		case <-time.After(time.Second):
			t.Fatal("fn tidak selesai setelah dilepas")
		}
	})

	t.Run("Level 2 - JalankanN (tiap goroutine punya i sendiri)", func(t *testing.T) {
		const n = 12
		var mu sync.Mutex
		dilihat := map[int]int{}
		hadir := 0
		gerbang := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(n)

		kembali := panggil(func() {
			JalankanN(n, func(i int) {
				defer wg.Done()
				mu.Lock()
				dilihat[i]++
				hadir++
				if hadir == n {
					close(gerbang)
				}
				mu.Unlock()
				select {
				case <-gerbang:
				case <-time.After(time.Second):
				}
			})
		})

		select {
		case r := <-kembali:
			if r != nil {
				t.Fatalf("JalankanN panic: %v (kemungkinan belum diimplementasikan)", r)
			}
		case <-time.After(time.Second):
			t.Fatal("JalankanN tidak kembali -- ia harus langsung kembali setelah meluncurkan goroutine")
		}
		selesai := make(chan struct{})
		go func() { wg.Wait(); close(selesai) }()
		select {
		case <-selesai:
		case <-time.After(3 * time.Second):
			t.Fatal("tidak semua fn(i) berjalan bersamaan/selesai -- masing-masing i harus dijalankan di goroutine sendiri")
		}
		mu.Lock()
		defer mu.Unlock()
		if len(dilihat) != n {
			t.Errorf("fn harus dipanggil untuk tiap i dari 0 sampai %d, terlihat %d nilai berbeda: %v", n-1, len(dilihat), dilihat)
		}
		for i := 0; i < n; i++ {
			if dilihat[i] != 1 {
				t.Errorf("fn(%d) dipanggil %d kali, ingin tepat 1 kali (variabel loop terbagi antar goroutine?)", i, dilihat[i])
			}
		}
	})

	t.Run("Level 3 - Unduh & UnduhBerurutan (pembanding)", func(t *testing.T) {
		var h Hasil
		mulai := time.Now()
		dalam(t, 2*time.Second, func() { h = Unduh(File{Nama: "a.zip", UkuranKB: 50, Durasi: 40 * time.Millisecond}) })
		if el := time.Since(mulai); el < 30*time.Millisecond {
			t.Errorf("Unduh harus benar-benar 'mengunduh' selama f.Durasi (time.Sleep), hanya makan %v", el)
		}
		if h.Nama != "a.zip" || h.UkuranKB != 50 || h.Err != nil {
			t.Errorf("Unduh sukses = %+v, ingin Nama a.zip, UkuranKB 50, Err nil", h)
		}

		for _, ukuran := range []int{0, -5} {
			dalam(t, 2*time.Second, func() { h = Unduh(File{Nama: "rusak", UkuranKB: ukuran, Durasi: time.Hour}) })
			if !errors.Is(h.Err, ErrUkuranTidakValid) {
				t.Errorf("Unduh file berukuran %d harus Err = ErrUkuranTidakValid (tanpa menunggu Durasi), dapat: %v", ukuran, h.Err)
			}
			if h.Nama != "rusak" {
				t.Errorf("Hasil gagal tetap harus memuat Nama file, dapat %q", h.Nama)
			}
		}

		files := []File{
			{Nama: "1", UkuranKB: 1, Durasi: 20 * time.Millisecond},
			{Nama: "2", UkuranKB: 2, Durasi: 20 * time.Millisecond},
			{Nama: "3", UkuranKB: 3, Durasi: 20 * time.Millisecond},
		}
		var hasil []Hasil
		mulai = time.Now()
		dalam(t, 3*time.Second, func() { hasil = UnduhBerurutan(files) })
		el := time.Since(mulai)
		if len(hasil) != 3 || hasil[0].Nama != "1" || hasil[1].Nama != "2" || hasil[2].Nama != "3" {
			t.Fatalf("UnduhBerurutan harus mengembalikan hasil sesuai urutan file, dapat %+v", hasil)
		}
		if el < 55*time.Millisecond {
			t.Errorf("UnduhBerurutan harus mengunduh SATU per satu (3 x 20ms >= 60ms), hanya makan %v", el)
		}
	})

	t.Run("Level 4 - UnduhSemua dengan WaitGroup (benar-benar bersamaan)", func(t *testing.T) {
		const n = 6
		p := barrierPelacak(n)
		var hasil []Hasil
		dalam(t, 3*time.Second, func() { hasil = UnduhSemua(buatFile(n), p.unduh) })
		puncak, total := p.hasilPuncak()
		if total != n {
			t.Errorf("unduh dipanggil %d kali, ingin %d (tiap file tepat sekali)", total, n)
		}
		if puncak != n {
			t.Errorf("hanya %d unduhan yang berjalan bersamaan pada puncaknya, ingin %d -- semua file harus diunduh bersamaan, masing-masing di goroutine sendiri", puncak, n)
		}
		if len(hasil) != n {
			t.Fatalf("UnduhSemua harus MENUNGGU semua unduhan selesai (WaitGroup.Wait) lalu mengembalikan %d hasil, dapat %d", n, len(hasil))
		}
		if !samaSlice(namaSet(hasil), namaFile(buatFile(n))) {
			t.Errorf("nama hasil tidak cocok dengan file yang diunduh: %v", namaSet(hasil))
		}
	})

	t.Run("Level 5 - UnduhSemua: urutan hasil & masukan kosong", func(t *testing.T) {
		const n = 8
		files := buatFile(n)
		// File pertama paling lambat selesai, yang terakhir paling cepat.
		lambatTerbalik := func(f File) Hasil {
			var i int
			fmt.Sscanf(f.Nama, "f%d", &i)
			time.Sleep(time.Duration(n-i) * 8 * time.Millisecond)
			return Hasil{Nama: f.Nama, UkuranKB: f.UkuranKB}
		}
		var hasil []Hasil
		dalam(t, 3*time.Second, func() { hasil = UnduhSemua(files, lambatTerbalik) })
		if len(hasil) != n {
			t.Fatalf("UnduhSemua mengembalikan %d hasil, ingin %d", len(hasil), n)
		}
		for i := range files {
			if hasil[i].Nama != files[i].Nama || hasil[i].UkuranKB != files[i].UkuranKB {
				t.Errorf("hasil[%d] = %+v, ingin file %q (urutan hasil HARUS mengikuti urutan files, bukan urutan selesai)", i, hasil[i], files[i].Nama)
			}
		}

		var kosong []Hasil
		dalam(t, time.Second, func() { kosong = UnduhSemua(nil, lambatTerbalik) })
		if kosong == nil || len(kosong) != 0 {
			t.Errorf("UnduhSemua(nil) harus mengembalikan slice kosong non-nil dan tidak macet, dapat %#v", kosong)
		}

		// Kegagalan satu file tidak boleh mengganggu yang lain.
		campur := []File{{Nama: "ok1", UkuranKB: 5}, {Nama: "bad", UkuranKB: 0}, {Nama: "ok2", UkuranKB: 7}}
		var h3 []Hasil
		dalam(t, 2*time.Second, func() { h3 = UnduhSemua(campur, Unduh) })
		if len(h3) != 3 || h3[0].Err != nil || h3[2].Err != nil || !errors.Is(h3[1].Err, ErrUkuranTidakValid) {
			t.Errorf("UnduhSemua dengan Unduh asli: satu file rusak harus Err sendiri, lainnya sukses; dapat %+v", h3)
		}
	})

	t.Run("Level 6 - UnduhSemua: panic di goroutine tidak mematikan program", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestAnakPanic$", "-test.v", "-test.count=1")
		cmd.Env = append(os.Environ(), "P5_ANAK=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			s := string(out)
			if len(s) > 900 {
				s = s[:900] + "..."
			}
			t.Fatalf("skenario 'satu unduhan panic' mematikan proses / gagal (%v). Ingat: recover() hanya menangkap panic di goroutine yang SAMA -- pasang defer+recover DI DALAM goroutine unduhan, ubah jadi Hasil.Err, dan pastikan wg.Done() tetap jalan (defer).\nkeluaran:\n%s", err, s)
		}
	})

	t.Run("Level 7 - UnduhKeChannel (channel hasil, ditutup pengirim)", func(t *testing.T) {
		const n = 6
		p := barrierPelacak(n)
		var ch <-chan Hasil
		dalam(t, time.Second, func() { ch = UnduhKeChannel(buatFile(n), p.unduh) })
		if ch == nil {
			t.Fatal("UnduhKeChannel mengembalikan channel nil")
		}

		var hasil []Hasil
		batas := time.After(3 * time.Second)
	kumpul:
		for {
			select {
			case h, ok := <-ch:
				if !ok {
					break kumpul
				}
				hasil = append(hasil, h)
			case <-batas:
				t.Fatalf("channel tidak pernah ditutup setelah %d hasil -- pengirim harus close(ch) setelah semua unduhan selesai (WaitGroup + goroutine penutup)", len(hasil))
			}
		}
		if len(hasil) != n {
			t.Fatalf("channel harus mengirim %d hasil, dapat %d", n, len(hasil))
		}
		if !samaSlice(namaSet(hasil), namaFile(buatFile(n))) {
			t.Errorf("nama hasil tidak lengkap/duplikat: %v", namaSet(hasil))
		}
		if puncak, _ := p.hasilPuncak(); puncak != n {
			t.Errorf("hanya %d unduhan bersamaan, ingin %d -- unduhan harus konkuren", puncak, n)
		}

		var kosong <-chan Hasil
		dalam(t, time.Second, func() { kosong = UnduhKeChannel(nil, Unduh) })
		select {
		case _, ok := <-kosong:
			if ok {
				t.Error("UnduhKeChannel(nil) harus mengembalikan channel yang sudah tertutup dan kosong")
			}
		case <-time.After(time.Second):
			t.Error("UnduhKeChannel(nil) harus mengembalikan channel yang langsung tertutup")
		}
	})

	t.Run("Level 8 - UnduhBatas (buffered channel sebagai semaphore)", func(t *testing.T) {
		const n, maks = 9, 3
		files := buatFile(n)
		p := barrierPelacak(maks)
		var hasil []Hasil
		var err error
		dalam(t, 5*time.Second, func() { hasil, err = UnduhBatas(files, p.unduh, maks) })
		if err != nil {
			t.Fatalf("UnduhBatas seharusnya tidak error, dapat: %v", err)
		}
		puncak, total := p.hasilPuncak()
		if puncak > maks {
			t.Errorf("puncak unduhan bersamaan = %d, melebihi batas %d", puncak, maks)
		}
		if puncak < maks {
			t.Errorf("puncak unduhan bersamaan = %d, seharusnya mencapai batas %d (jangan terlalu membatasi)", puncak, maks)
		}
		if total != n || len(hasil) != n {
			t.Fatalf("semua %d file harus diunduh; unduh dipanggil %d kali, hasil %d", n, total, len(hasil))
		}
		for i := range files {
			if hasil[i].Nama != files[i].Nama {
				t.Errorf("hasil[%d].Nama = %q, ingin %q (urutan mengikuti files)", i, hasil[i].Nama, files[i].Nama)
			}
		}

		// Batas lebih besar dari jumlah file: semua boleh jalan bersamaan.
		p2 := barrierPelacak(4)
		dalam(t, 3*time.Second, func() { hasil, err = UnduhBatas(buatFile(4), p2.unduh, 100) })
		if puncak, _ := p2.hasilPuncak(); err != nil || puncak != 4 {
			t.Errorf("batas 100 untuk 4 file: puncak = %d (ingin 4), err = %v", puncak, err)
		}

		// Batas tidak valid.
		p3 := barrierPelacak(0)
		for _, buruk := range []int{0, -1} {
			dalam(t, time.Second, func() { _, err = UnduhBatas(buatFile(2), p3.unduh, buruk) })
			if !errors.Is(err, ErrBatasTidakValid) {
				t.Errorf("UnduhBatas dengan maks=%d harus ErrBatasTidakValid, dapat: %v", buruk, err)
			}
		}
		if _, total := p3.hasilPuncak(); total != 0 {
			t.Errorf("batas tidak valid tidak boleh memulai unduhan apa pun, tapi unduh dipanggil %d kali", total)
		}
	})

	t.Run("Level 9 - GabungChannel (fan-in)", func(t *testing.T) {
		produsen := func(nama string, jumlah int, tunda time.Duration) <-chan Hasil {
			ch := make(chan Hasil)
			go func() {
				defer close(ch)
				time.Sleep(tunda)
				for i := 0; i < jumlah; i++ {
					ch <- Hasil{Nama: fmt.Sprintf("%s%d", nama, i)}
				}
			}()
			return ch
		}

		var gabung <-chan Hasil
		dalam(t, time.Second, func() {
			gabung = GabungChannel(produsen("a", 3, 0), produsen("b", 0, 0), produsen("c", 5, 120*time.Millisecond))
		})

		var nama []string
		batas := time.After(3 * time.Second)
	kumpul:
		for {
			select {
			case h, ok := <-gabung:
				if !ok {
					break kumpul
				}
				nama = append(nama, h.Nama)
			case <-batas:
				t.Fatalf("channel gabungan tidak ditutup setelah %d hasil -- tutup HANYA setelah semua channel masukan tertutup", len(nama))
			}
		}
		sort.Strings(nama)
		want := []string{"a0", "a1", "a2", "c0", "c1", "c2", "c3", "c4"}
		if !samaSlice(nama, want) {
			t.Errorf("hasil gabungan = %v, ingin %v (channel ditutup terlalu cepat, atau ada yang hilang?)", nama, want)
		}

		var kosong <-chan Hasil
		dalam(t, time.Second, func() { kosong = GabungChannel() })
		select {
		case _, ok := <-kosong:
			if ok {
				t.Error("GabungChannel() tanpa argumen harus mengembalikan channel tertutup dan kosong")
			}
		case <-time.After(time.Second):
			t.Error("GabungChannel() tanpa argumen harus langsung menutup channel hasil")
		}
	})

	t.Run("Level 10 - Progres (Mutex), Ringkasan & analisis", func(t *testing.T) {
		const goroutine, per = 100, 500
		var p *Progres
		dalam(t, time.Second, func() { p = NewProgres(goroutine * per) })

		var wg sync.WaitGroup
		wg.Add(goroutine)
		dalam(t, 20*time.Second, func() {
			for g := 0; g < goroutine; g++ {
				go func(g int) {
					defer wg.Done()
					for i := 0; i < per; i++ {
						if (g+i)%5 == 0 {
							p.Catat(Hasil{Nama: "x", UkuranKB: 0, Err: ErrUkuranTidakValid})
						} else {
							p.Catat(Hasil{Nama: "x", UkuranKB: 2})
						}
					}
				}(g)
			}
			wg.Wait()
		})
		var r Ringkasan
		dalam(t, time.Second, func() { r = p.Snapshot() })
		gagalWant := goroutine * per / 5
		sukses := goroutine*per - gagalWant
		if r.Total != goroutine*per || r.Gagal != gagalWant || r.Selesai != sukses || r.TotalKB != sukses*2 {
			t.Errorf("Snapshot setelah %d Catat bersamaan = %+v; ingin Total=%d Selesai=%d Gagal=%d TotalKB=%d (ada update yang hilang? -- lindungi penghitung dengan sync.Mutex)",
				goroutine*per, r, goroutine*per, sukses, gagalWant, sukses*2)
		}

		// Unduhan gagal tidak menambah TotalKB.
		q := NewProgres(3)
		dalam(t, time.Second, func() {
			q.Catat(Hasil{Nama: "a", UkuranKB: 100})
			q.Catat(Hasil{Nama: "b", UkuranKB: 250, Err: ErrUkuranTidakValid})
			q.Catat(Hasil{Nama: "c", UkuranKB: 50})
		})
		want := Ringkasan{Total: 3, Selesai: 2, Gagal: 1, TotalKB: 150}
		if got := q.Snapshot(); got != want {
			t.Errorf("Snapshot = %+v, ingin %+v (unduhan gagal tidak dihitung ke TotalKB)", got, want)
		}
		if got := fmt.Sprint(want); got != "2 dari 3 berhasil, 1 gagal, total 150 KB" {
			t.Errorf("String() ringkasan = %q, ingin %q", got, "2 dari 3 berhasil, 1 gagal, total 150 KB")
		}

		s := section(readFile("../README.md"), "Analisis Pertemuan 5")
		if !filled(s, 200) {
			t.Fatalf("section '## Analisis Pertemuan 5' di README.md (root repo) tidak ada atau belum diisi memadai (minimal 200 karakter, saat ini %d) -- tambahkan heading persis itu kalau belum ada", len(strings.TrimSpace(s)))
		}
		if !mengandungSalahSatu(s, "race", "balapan") {
			t.Error("analisis harus menjelaskan race condition (kata 'race' / 'balapan')")
		}
		if !mengandungSalahSatu(s, "mutex", "lock", "kunci") {
			t.Error("analisis harus menyinggung Mutex/lock")
		}
		if !mengandungSalahSatu(s, "indeks", "index", "elemen") {
			t.Error("analisis harus membahas kenapa menulis hasil[i] (indeks berbeda) per goroutine tidak butuh Mutex")
		}
	})
}
