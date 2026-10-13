# Pertemuan 6 — Task Manager v2: REST API CRUD & JSON

Sekarang Task Manager punya wajah HTTP sungguhan: **REST API CRUD** di atas `TaskStore`. Ini pembuktian kenapa kita membuat interface di Pertemuan 4 — handler HTTP kalian hanya bicara ke `TaskStore` (interface), tidak tahu-menahu soal `MemoryStore`. Di Pertemuan 7 kita menukar isinya dengan PostgreSQL tanpa menyentuh handler.

## Struktur folder

```
pertemuan-06/
├── go.mod        # sudah ada -- 'go 1.22' WAJIB (lihat catatan di bawah)
├── store.go      # SUDAH LENGKAP, jangan diubah: Task, TaskStore (+Update), MemoryStore
├── api.go        # <-- YANG KALIAN KERJAKAN: routing + handler + writeJSON + writeError
├── main.go       # sudah ada (menyalakan server di :8080)
└── main_test.go  # test grading, jangan diubah
```

Semua level dicek lewat `httptest` terhadap `NewAPI(store).Routes()` — tidak perlu membuka port. Kalau ingin mencoba sungguhan: `go run .` lalu `curl.exe` (di PowerShell Windows pakai `curl.exe`, bukan `curl` yang merupakan alias lain; body JSON dari file: `curl.exe -d "@body.json" -H "Content-Type: application/json" http://localhost:8080/tasks`).

**Catatan `go.mod`:** pola routing `"GET /tasks/{id}"` baru dikenali Go **1.22 ke atas**. Kalau baris `go` di `go.mod` lebih rendah, route diam-diam tidak cocok dan semuanya menjawab 404 tanpa pesan error apa pun. `go.mod` di folder ini sudah `go 1.22` — jangan diturunkan.

Ada satu section README yang dicek di Level 10: heading persis `## Dokumentasi API Task Manager` — **tambahkan sendiri** di `README.md` root repo kalian (belum ada di repo yang dibuat dari template lama).

## Bentuk data (sudah ada di starter)

```go
type Task struct { ID int `json:"id"`; Judul string `json:"judul"`; Selesai bool `json:"selesai"` }
type taskInput struct { Judul string `json:"judul"`; Selesai bool `json:"selesai"` } // isi body POST/PUT (tanpa ID!)
type errorBody struct { Error string `json:"error"` }                                // {"error":"pesan"}
```

---

## Level 1 — `GET /tasks` & `writeJSON`

```go
func (a *API) Routes() *http.ServeMux
func writeJSON(w http.ResponseWriter, status int, v any)
func (a *API) listTasks(w http.ResponseWriter, r *http.Request)
```

`Routes()` mendaftarkan endpoint dengan pola `"METHOD /path"` (mis. `mux.HandleFunc("GET /tasks", a.listTasks)`). `GET /tasks` → status **200**, `Content-Type: application/json`, body = array JSON semua tugas sesuai urutan store, tiap tugas dengan kunci `id`, `judul`, `selesai`. Store kosong harus menghasilkan `[]`, **bukan** `null`.

`writeJSON` adalah helper yang akan kalian pakai di semua handler. **Urutannya wajib**: (1) `w.Header().Set("Content-Type", "application/json")`, (2) `w.WriteHeader(status)`, (3) encode body. Header yang di-set *setelah* `WriteHeader` tidak pernah terkirim — test memeriksa header seperti yang benar-benar diterima klien, jadi urutan yang salah akan ketahuan.

## Level 2 — `POST /tasks`

```go
func (a *API) createTask(w http.ResponseWriter, r *http.Request)
```

Decode body JSON ke `taskInput` (`json.NewDecoder(r.Body).Decode(&in)`), simpan lewat `store.Add(Task{Judul: in.Judul, Selesai: in.Selesai})`, jawab **201 Created** dengan Task yang baru dibuat (lengkap dengan `id` dari store). Body `{"judul":"x"}` tanpa `selesai` → `selesai:false`.

## Level 3 — `GET /tasks/{id}`

```go
func (a *API) getTask(w http.ResponseWriter, r *http.Request)
```

Ambil id dari path dengan `r.PathValue("id")`, ubah ke `int` (`strconv.Atoi`), cari lewat `store.Get`, jawab **200** + Task. (Kasus id tidak ada / bukan angka ada di Level 6–7.)

## Level 4 — `PUT /tasks/{id}` (penggantian penuh)

```go
func (a *API) updateTask(w http.ResponseWriter, r *http.Request)
```

`PUT` = **mengganti seluruh** representasi tugas dengan body yang dikirim: `store.Update(Task{ID: id, Judul: in.Judul, Selesai: in.Selesai})`, jawab **200** + Task terbaru. Konsekuensinya: field yang tidak dikirim kembali ke nilai nol — `PUT {"judul":"Baru"}` pada tugas yang sudah selesai membuatnya **belum selesai** lagi (test memeriksa ini). Tugas lain tidak boleh tersentuh.

## Level 5 — `DELETE /tasks/{id}`

```go
func (a *API) deleteTask(w http.ResponseWriter, r *http.Request)
```

Hapus lewat `store.Delete`, jawab **204 No Content** tanpa body. Menghapus id yang sudah tidak ada → 404 (pengecekan status saja di level ini).

## Level 6 — Kode status untuk kasus gagal

Semua kasus di bawah harus menghasilkan status yang tepat, di **semua endpoint yang relevan**:

| Situasi | Status |
|---|---|
| `id` tidak ada di store (GET/PUT/DELETE) | **404** |
| `id` di path bukan angka (mis. `/tasks/abc`) | **400** |
| body bukan JSON / kosong / tipe field salah (`{"judul":123}`) (POST/PUT) | **400** |
| judul kosong atau hanya spasi (POST/PUT) | **400** |

Pikirkan: kenapa `/tasks/abc` itu 400, bukan 404? (Jawabannya kalian butuhkan lagi di Level 10.) Tulis `writeError(w, err)` untuk memetakan error aplikasi ke status: `ErrTugasTidakDitemukan` → 404, `ErrInputKosong` → 400, lainnya → 500 (Level 8). Request gagal **tidak boleh mengubah isi store**.

## Level 7 — Body error terstruktur

```go
func writeError(w http.ResponseWriter, err error)
```

Semua respons error harus **JSON** `{"error":"<pesan>"}` dengan `Content-Type: application/json` — bukan teks polos dari `http.Error`. Pesan yang dicek **persis**:

| Situasi | `error` |
|---|---|
| tugas tidak ada | `tugas tidak ditemukan` |
| id bukan angka | `id harus berupa angka` |
| body bukan JSON / tipe salah / kosong | `body bukan JSON yang valid` |
| judul kosong | `input tidak boleh kosong` |

`writeError` harus memakai **`errors.Is`**, bukan `==`: test memanggilnya dengan `fmt.Errorf("get tugas 7: %w", ErrTugasTidakDitemukan)` (error yang dibungkus) dan tetap mengharapkan 404 — di Pertemuan 7 error dari database akan selalu dibungkus seperti ini.

## Level 8 — Error internal: 500 generik, tanpa bocor

Kalau `TaskStore` gagal dengan error yang **tidak dikenal** (mis. koneksi database putus), jawab **500** dengan pesan generik `{"error":"terjadi kesalahan internal"}`. Test menyuntikkan store yang gagal dengan pesan `"koneksi db meledak: password=rahasia123 host=10.0.0.5"` dan memastikan **tidak ada** kata "rahasia", "password", alamat IP, dsb. yang muncul di body respons. Detail asli dicatat ke **log server** (`log.Printf`), bukan dikirim ke klien — pesan error mentah bisa membocorkan struktur internal sistem (nama tabel, kredensial, versi) kepada penyerang.

## Level 9 — Kontrak API: ID bukan urusan klien

- `POST` dengan body `{"id":999,"judul":"Curang"}` → `id` di body **diabaikan**; ID hanya boleh dibuat oleh store (itulah gunanya `taskInput` tidak punya field ID).
- `PUT /tasks/{id}` dengan `id` di body yang berbeda dari path → **path yang menang**; tugas ber-ID lain tidak boleh tertimpa.
- JSON `Task` punya **persis** tiga kunci (`id`, `judul`, `selesai`) dengan tipe yang benar (angka, string, boolean).
- Metode/rute yang tidak didukung (`PATCH /tasks/1`, `POST /tasks/1`, `GET /nope`) ditolak dengan **4xx** — gratis kalau `Routes()` memakai pola `"METHOD /path"` (mux menjawab 405/404 sendiri; body-nya teks biasa dan itu tidak dicek).

Pertanyaan penalaran: apa yang bisa salah kalau kita men-decode body langsung ke `Task` (yang punya `ID`) alih-alih ke `taskInput`?

## Level 10 — Dokumentasi endpoint di README

Tambahkan section `## Dokumentasi API Task Manager` di `README.md` (root repo) — tabel ringkas + contoh, **minimal ±500 karakter**, memuat:

- kelima endpoint (`GET /tasks`, `POST /tasks`, `GET /tasks/{id}`, `PUT /tasks/{id}`, `DELETE /tasks/{id}`) beserta method dan path-nya,
- kode status sukses & gagal yang dipakai (200, 201, 204, 400, 404, 500),
- minimal satu contoh body request (`{"judul": "..."}`) dan satu contoh respons error (`{"error": "..."}`),
- penjelasan singkat, dengan kata-kata sendiri: **kenapa** id bukan angka itu 400 dan bukan 404, dan kenapa error 500 tidak menampilkan detail aslinya ke klien (sebut *log* atau *internal*).

**Dicek otomatis:** kelima `METHOD path` muncul, semua kode status tersebut disebut, ada contoh `"judul"` dan `"error"`, dan ada kata *log*/*internal*. Ini hanya memastikan dokumen benar-benar terisi — kejelasan penjelasannya dibaca dosen.
