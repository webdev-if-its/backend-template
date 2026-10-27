package main

// ValidationError membawa alasan penolakan per field, mis.
// {"judul": "tidak boleh kosong"}. (Level 7)
type ValidationError struct {
	Fields map[string]string
}

// Error mengembalikan teks "validasi gagal" (supaya *ValidationError memenuhi
// interface error). (Level 7)
func (e *ValidationError) Error() string {
	panic("belum diimplementasikan")
}

// ValidasiTugas memeriksa input tugas. Kembalikan nil kalau valid, atau
// *ValidationError berisi SEMUA field yang bermasalah. (Level 7)
func ValidasiTugas(in taskInput) error {
	panic("belum diimplementasikan")
}
