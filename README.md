# backend-nrp

Repo tugas mata kuliah **Pengembangan Backend Dasar**, dibuat dari template [`webdev-if-its/backend-template`](https://github.com/webdev-if-its/backend-template). Ganti judul di atas jadi nama repo kalian sendiri (`backend-nrp`, contoh: `backend-5025201012`).

## Aturan Umum

- Tugas tiap pertemuan disimpan di folder `pertemuan-XX/` pada repo ini.
- Commit message wajib menyebut level yang dicapai: `pertemuan-XX: level N selesai`.
- Deadline push: sebelum pertemuan berikutnya dimulai.
- Semua level dicek otomatis lewat `go test` — baca `pertemuan-XX/SOAL.md` tiap minggu untuk detail levelnya.

## Mengambil Pertemuan Baru Tiap Minggu

Repo ini **tidak otomatis sinkron** dengan template dosen. Begitu ada pertemuan baru, jalankan (ganti `pertemuan-02` sesuai minggu berjalan):

```bash
git fetch https://github.com/webdev-if-its/backend-template.git main
git checkout FETCH_HEAD -- pertemuan-02
```

Perintah ini **aman dijalankan kapan pun** — tidak akan menimpa folder pertemuan lain yang sudah kalian kerjakan, karena hanya mengambil folder yang disebutkan. Setelah itu, commit folder barunya seperti biasa.

Kalau dosen memperbaiki sesuatu di pertemuan yang sudah dirilis (mis. ada bug di test), biasanya cukup ambil ulang file yang diperbaiki saja, bukan seluruh folder — akan diumumkan file mana yang berubah.

---

Bagian di bawah ini **isi bertahap** sesuai level yang sedang kalian kerjakan (lihat `pertemuan-XX/SOAL.md` yang menyebutnya) — heading-nya dicek otomatis, jangan diganti namanya. Repo kalian dibuat dari template sekali di awal semester, jadi section README untuk pertemuan yang datang belakangan **tidak otomatis muncul** di repo kalian: kalau `SOAL.md` sebuah pertemuan meminta section baru, tambahkan sendiri heading-nya (`## ...` persis seperti yang disebut di soal) ke `README.md` repo kalian.

## Identitas
- Nama: (tulis di sini)
- NRP: (tulis di sini)
- Kelas: (tulis di sini)

## Commit vs Push
(tulis di sini)

## Reproducibility
(tulis di sini)

## Catatan Merge Conflict
(tulis di sini)

## Kenapa .gitignore Penting
(tulis di sini)

## Refleksi
(tulis di sini)

## Analisis Pertemuan 4
(tulis di sini)

## Analisis Pertemuan 5
(tulis di sini)

## Dokumentasi API Task Manager
(tulis di sini)

## Analisis Pertemuan 7
(tulis di sini)

## Analisis Pertemuan 9
(tulis di sini)

## Arsitektur Task Manager v5
(tulis di sini)

## Checklist Keamanan Task Manager v6
(tulis di sini)

## Refleksi Topik Lanjutan
(tulis di sini)
