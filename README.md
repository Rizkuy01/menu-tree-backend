# Menu Tree Backend

Backend REST API untuk mengelola struktur menu hierarkis dengan kedalaman tidak
terbatas. Project ini dibuat sebagai bagian dari technical test Fullstack Menu
Tree System.

API dibangun dengan Go dan Gin, menggunakan PostgreSQL sebagai database, UUID
sebagai identifier, serta adjacency list untuk menyimpan relasi parent-child.
Selain CRUD dasar, backend ini menyediakan endpoint bonus untuk memindahkan menu
beserta seluruh subtree-nya (`move`) dan mengatur urutan antar-sibling
(`reorder`).

## Daftar Isi

- [Fitur](#fitur)
- [Teknologi dan alasan pemilihan](#teknologi-dan-alasan-pemilihan)
- [Struktur project](#struktur-project)
- [Prasyarat](#prasyarat)
- [Instalasi](#instalasi)
- [Menjalankan server](#menjalankan-server)
- [Swagger / OpenAPI](#swagger--openapi)
- [Konsep data dan aliran data](#konsep-data-dan-aliran-data)
- [Kontrak API](#kontrak-api)
- [Testing API dengan curl](#testing-api-dengan-curl)
- [Testing dan quality checks](#testing-dan-quality-checks)
- [Validasi dan error handling](#validasi-dan-error-handling)
- [Keputusan arsitektur](#keputusan-arsitektur)
- [Troubleshooting](#troubleshooting)
- [Status technical test](#status-technical-test)

## Fitur

- Membuat menu root atau child pada level mana pun.
- Mengambil seluruh menu sebagai nested tree melalui field `children`.
- Mengambil detail satu menu, termasuk nama parent.
- Mengubah nama menu.
- Menghapus menu beserta seluruh descendant menggunakan foreign key cascade.
- Memindahkan menu dan subtree ke parent lain atau kembali ke root.
- Mengatur posisi menu di antara sibling.
- Menyimpan `depth` dan `position` untuk mendukung kebutuhan UI tree.
- Validasi UUID, nama, parent, dan posisi.
- Pencegahan cycle: menu tidak dapat dipindahkan ke dirinya sendiri atau
  descendant-nya.
- Swagger UI dan dokumen OpenAPI JSON/YAML.
- Unit test service untuk aturan posisi move/reorder.
- Query SQL menggunakan parameter, bukan string concatenation dari input user.

## Teknologi dan alasan pemilihan

| Area            | Teknologi                     | Alasan                                                                 |
| --------------- | ----------------------------- | ---------------------------------------------------------------------- |
| Bahasa          | Go                            | Ringan, cepat, type-safe, dan cocok untuk REST API                     |
| HTTP            | Gin                           | Routing dan binding JSON sederhana serta performa baik                 |
| Database        | PostgreSQL                    | Mendukung UUID, recursive CTE, transaksi, dan foreign key              |
| Driver          | `pgx/v5` + `pgxpool`          | Driver PostgreSQL native dengan connection pooling                     |
| Migrasi         | SQL migration                 | Schema dapat direproduksi dan dilacak melalui versioning               |
| Validasi        | Gin binding / validator       | Validasi format request dilakukan dekat boundary HTTP                  |
| Dokumentasi API | Swaggo                        | Swagger UI dapat digunakan evaluator untuk mencoba endpoint            |
| Identifier      | UUID                          | Tidak bergantung pada urutan ID dan aman digunakan sebagai resource ID |
| Konfigurasi     | Environment variable + `.env` | Memisahkan konfigurasi dari source code                                |

## Struktur project

```text
menu-tree-backend/
├── cmd/api/main.go                 # entry point dan metadata Swagger
├── internal/
│   ├── config/config.go            # load PORT dan DATABASE_URL
│   ├── database/database.go        # membuat dan ping connection pool
│   ├── dto/menu.go                 # request DTO dan validation tags
│   ├── handler/
│   │   ├── menu.go                 # parsing request dan response HTTP
│   │   └── response.go             # format serta mapping error
│   ├── model/
│   │   ├── menu.go                 # model response dan MenuNode
│   │   └── errors.go               # sentinel errors
│   ├── repository/menu.go          # SQL dan transaksi database
│   ├── router/router.go            # route, CORS, health check, Swagger
│   └── service/menu.go             # business rules dan pembentukan tree
├── migrations/
│   ├── 000001_create_menus.up.sql
│   └── 000001_create_menus.down.sql
├── docs/                           # generated Swagger files
├── .env.example
├── go.mod
└── README.md
```

Layering utama adalah:

```text
HTTP Client
    │
    ▼
Router ──► Handler ──► Service ──► Repository ──► PostgreSQL
              │           │             │
              │           │             └── SQL parameterized + transaction
              │           └── aturan bisnis tree, depth, dan posisi
              └── bind JSON, parse UUID, status code, response
```

Handler tidak berisi SQL dan service tidak bergantung pada tipe HTTP. Service
bergantung pada interface repository sehingga dapat diuji dengan mock/stub tanpa
database sungguhan.

## Prasyarat

Install komponen berikut sebelum menjalankan project:

1. Go sesuai versi pada [`go.mod`](./go.mod) (saat ini `go 1.27.2`).
2. PostgreSQL 13+ atau database PostgreSQL-compatible seperti Neon.
3. `golang-migrate` CLI untuk menjalankan migration.
4. Git Bash atau terminal PowerShell pada Windows.
5. `swag` CLI hanya jika ingin regenerate dokumentasi Swagger.

Docker tidak diperlukan untuk menjalankan backend ini dan tidak digunakan dalam
validasi lokal project.

## Instalasi

### 1. Clone dan masuk ke folder project

```bash
git clone <URL_REPOSITORY>
cd menu-tree-backend
```

### 2. Download dependency Go

```bash
go mod download
```

### 3. Siapkan database

Buat database PostgreSQL baru. Untuk Neon, gunakan **direct connection string**
(bukan pooled connection string) dan pastikan connection string memiliki
`sslmode=require`.

Salin file environment:

```bash
cp .env.example .env
```

Kemudian isi `.env` dengan nilai nyata:

```dotenv
PORT=8080
DATABASE_URL=postgres://USER:PASSWORD@HOST/DATABASE?sslmode=require
```

Jangan commit `.env`. File tersebut sudah di-ignore oleh Git; gunakan
[`.env.example`](./.env.example) sebagai template tanpa secret.

### 4. Install `golang-migrate`

Contoh instalasi melalui Go:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Pastikan folder binary Go ada di `PATH`. Verifikasi:

```bash
migrate -version
```

### 5. Jalankan migration

Di Git Bash:

```bash
export $(grep DATABASE_URL .env | xargs)
migrate -path migrations -database "$DATABASE_URL" up
```

Di PowerShell:

```powershell
$env:DATABASE_URL = (Get-Content .env | Where-Object { $_ -match '^DATABASE_URL=' }) -replace '^DATABASE_URL=', ''
migrate -path migrations -database $env:DATABASE_URL up
```

Migration membuat tabel `menus`, foreign key self-reference, dan index untuk
parent serta posisi sibling.

Untuk rollback satu migration:

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

> Perintah `down` menghapus tabel `menus` dan seluruh data di dalamnya.

## Menjalankan server

```bash
go run ./cmd/api
```

Server berjalan pada:

- Health check: <http://localhost:8080/health>
- API base URL: <http://localhost:8080/api>
- Swagger UI: <http://localhost:8080/swagger/index.html>
- OpenAPI JSON: <http://localhost:8080/swagger.json>

Port dapat diubah melalui `PORT`.

Contoh health check:

```bash
curl http://localhost:8080/health
```

Response:

```json
{ "status": "ok" }
```

## Swagger / OpenAPI

Swagger UI tersedia di `/swagger/index.html` dan dapat digunakan untuk
menjalankan request langsung dari browser. File OpenAPI yang dihasilkan berada
di [`docs/`](./docs/).

Jika annotation, DTO, model, atau route berubah, regenerate dengan:

```bash
swag init -g cmd/api/main.go -o docs
```

Dokumen Swagger adalah hasil generate; jangan mengedit
[`docs/swagger.yaml`](./docs/swagger.yaml) atau
[`docs/swagger.json`](./docs/swagger.json) secara manual.

## Konsep data dan aliran data

### Model adjacency list

Setiap menu disimpan sebagai satu row:

| Kolom        | Arti                                                     |
| ------------ | -------------------------------------------------------- |
| `id`         | UUID unik menu                                           |
| `name`       | Nama menu, wajib, maksimal 255 karakter                  |
| `parent_id`  | UUID parent; `NULL` berarti root                         |
| `depth`      | Root bernilai `0`, child bernilai `parent.depth + 1`     |
| `position`   | Urutan 0-based di antara sibling dengan parent yang sama |
| `created_at` | Waktu pembuatan                                          |
| `updated_at` | Waktu perubahan terakhir                                 |

Contoh data:

```text
Systems             parent_id=NULL, depth=0, position=0
├── System Code      parent_id=Systems, depth=1, position=0
└── Properties       parent_id=Systems, depth=1, position=1
    └── General      parent_id=Properties, depth=2, position=0
```

Pendekatan ini lebih sederhana daripada menyimpan JSON tree di database:
perubahan satu node tetap terlokalisasi, relasi dapat di-index, dan PostgreSQL
recursive CTE dapat digunakan untuk validasi cycle serta update depth subtree.

### Alur request normal

1. Client mengirim HTTP request ke route Gin.
2. Router meneruskan request ke handler sesuai method dan path.
3. Handler mem-parse UUID path dan bind body JSON ke DTO.
4. Validation tag menolak body yang formatnya tidak valid.
5. Service menerapkan aturan bisnis, misalnya trim nama, menghitung depth,
   memeriksa posisi negatif, atau mengambil parent.
6. Service memanggil `MenuRepository` interface.
7. Repository menjalankan SQL parameterized menggunakan `pgxpool`.
8. Hasil database dikembalikan melewati service lalu handler.
9. Handler mengembalikan JSON dengan HTTP status yang sesuai.
10. Error domain dipetakan ke format `{ "error": { "code", "message" } }`.

### Alur GET tree

`GET /api/menus` mengambil semua row dengan urutan posisi, lalu service:

1. Membuat map `id -> MenuNode`.
2. Membuat setiap node dengan `children` array kosong.
3. Menambahkan node tanpa `parent_id` ke daftar roots.
4. Menambahkan node lain ke `children` parent-nya.
5. Mengembalikan hanya roots dengan descendants nested.

Dengan cara ini, database menyimpan adjacency list sedangkan frontend menerima
struktur tree yang langsung dapat dirender secara rekursif.

### Alur create

1. Nama di-trim dan ditolak jika kosong.
2. Jika `parent_id` ada, parent dicari.
3. `depth` dihitung dari parent.
4. `position` otomatis menjadi posisi terakhir sibling.
5. Row baru di-insert.

### Alur move dan reorder

`reorder` menggunakan fungsi repository yang sama dengan `move`, tetapi parent
yang dipakai adalah parent saat ini.

Operasi move dijalankan atomik dalam satu transaksi:

1. Mengambil advisory transaction lock untuk serialisasi perubahan struktur.
2. Memuat menu sumber dan target parent.
3. Menjalankan recursive CTE untuk menolak target yang merupakan diri sendiri
   atau descendant.
4. Menutup gap pada posisi sibling lama.
5. Menghitung posisi baru; posisi tanpa nilai berarti append, nilai terlalu besar
   di-clamp ke posisi terakhir.
6. Membuka gap di sibling baru.
7. Mengubah `parent_id`, `position`, dan `depth` menu.
8. Jika depth berubah, delta yang sama diterapkan ke semua descendant.
9. Commit; jika ada error sebelum commit, transaksi di-rollback.

Pendekatan ini menjaga urutan sibling tetap konsisten dan mencegah subtree
menjadi cyclic.

## Kontrak API

Semua endpoint menu menggunakan prefix `/api/menus`.

| Method   | Endpoint                 | Body                      | Hasil                               |
| -------- | ------------------------ | ------------------------- | ----------------------------------- |
| `GET`    | `/api/menus`             | -                         | Array root dengan `children` nested |
| `GET`    | `/api/menus/:id`         | -                         | Detail menu dan `parent_name`       |
| `POST`   | `/api/menus`             | `{name, parent_id?}`      | Membuat root/child, `201`           |
| `PUT`    | `/api/menus/:id`         | `{name}`                  | Mengubah nama                       |
| `DELETE` | `/api/menus/:id`         | -                         | Menghapus subtree, `204`            |
| `PATCH`  | `/api/menus/:id/move`    | `{parent_id?, position?}` | Memindahkan subtree                 |
| `PATCH`  | `/api/menus/:id/reorder` | `{position}`              | Mengubah urutan sibling             |

`parent_id: null` pada `move` berarti memindahkan menu ke root. Field `position`
pada `move` boleh dihilangkan untuk append ke akhir; pada `reorder` wajib ada.
Semua posisi menggunakan indeks mulai dari `0`.

## Testing API dengan curl

Jalankan server terlebih dahulu, lalu gunakan Git Bash. Simpan ID dari response
ke variable agar request berikutnya mudah diulang:

```bash
BASE_URL=http://localhost:8080

# Health check
curl -i "$BASE_URL/health"

# Membuat root
ROOT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/menus" \
  -H "Content-Type: application/json" \
  -d '{"name":"Systems"}')
echo "$ROOT_RESPONSE"
ROOT_ID=$(printf '%s' "$ROOT_RESPONSE" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

# Membuat dua child
CODE_RESPONSE=$(curl -s -X POST "$BASE_URL/api/menus" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"System Code\",\"parent_id\":\"$ROOT_ID\"}")
CODE_ID=$(printf '%s' "$CODE_RESPONSE" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

PROPERTY_RESPONSE=$(curl -s -X POST "$BASE_URL/api/menus" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Properties\",\"parent_id\":\"$ROOT_ID\"}")
PROPERTY_ID=$(printf '%s' "$PROPERTY_RESPONSE" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

# Melihat tree lengkap
curl -i "$BASE_URL/api/menus"

# Melihat detail satu menu
curl -i "$BASE_URL/api/menus/$CODE_ID"

# Mengubah nama
curl -i -X PUT "$BASE_URL/api/menus/$CODE_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"System Configuration"}'

# Reorder: pindahkan child ke posisi pertama
curl -i -X PATCH "$BASE_URL/api/menus/$PROPERTY_ID/reorder" \
  -H "Content-Type: application/json" \
  -d '{"position":0}'

# Move: pindahkan menu ke parent lain; tanpa position berarti append
curl -i -X PATCH "$BASE_URL/api/menus/$CODE_ID/move" \
  -H "Content-Type: application/json" \
  -d "{\"parent_id\":\"$PROPERTY_ID\"}"

# Move kembali ke root pada posisi pertama
curl -i -X PATCH "$BASE_URL/api/menus/$CODE_ID/move" \
  -H "Content-Type: application/json" \
  -d '{"parent_id":null,"position":0}'

# Delete: parent dan semua descendant ikut terhapus
curl -i -X DELETE "$BASE_URL/api/menus/$CODE_ID"
```

### Skenario validasi dan error

```bash
# UUID tidak valid -> 400 INVALID_ID
curl -i "$BASE_URL/api/menus/not-a-uuid"

# Nama kosong -> 400 VALIDATION_ERROR
curl -i -X POST "$BASE_URL/api/menus" \
  -H "Content-Type: application/json" \
  -d '{"name":"   "}'

# Posisi negatif -> 400 VALIDATION_ERROR
curl -i -X PATCH "$BASE_URL/api/menus/$ROOT_ID/reorder" \
  -H "Content-Type: application/json" \
  -d '{"position":-1}'

# Parent tidak ada -> 404 PARENT_NOT_FOUND
curl -i -X POST "$BASE_URL/api/menus" \
  -H "Content-Type: application/json" \
  -d '{"name":"Orphan","parent_id":"00000000-0000-0000-0000-000000000000"}'

# Cycle: mencoba memindahkan parent ke descendant -> 409 INVALID_MOVE
curl -i -X PATCH "$BASE_URL/api/menus/$ROOT_ID/move" \
  -H "Content-Type: application/json" \
  -d "{\"parent_id\":\"$PROPERTY_ID\"}"
```

Contoh response error:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "position must be zero or greater"
  }
}
```

## Testing dan quality checks

Jalankan seluruh unit test:

```bash
go test ./...
```

Test yang tersedia mencakup penolakan posisi negatif pada service `move` dan
`reorder` melalui stub repository, sehingga aturan tersebut dapat diuji tanpa
database eksternal.

Pemeriksaan yang direkomendasikan sebelum submission:

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./...
```

Jika `gofmt -l .` menghasilkan nama file, format ulang file tersebut dengan:

```bash
gofmt -w .
```

Pengujian integrasi API memerlukan PostgreSQL yang aktif, migration yang sudah
dijalankan, dan server yang sedang berjalan. Docker tidak diasumsikan tersedia
di environment developer; gunakan PostgreSQL lokal atau Neon.

## Validasi dan error handling

| HTTP  | Code               | Kondisi                                            |
| ----- | ------------------ | -------------------------------------------------- |
| `400` | `VALIDATION_ERROR` | Body invalid, nama kosong, atau posisi negatif     |
| `400` | `INVALID_ID`       | Path ID bukan UUID                                 |
| `404` | `NOT_FOUND`        | Menu tidak ditemukan                               |
| `404` | `PARENT_NOT_FOUND` | Parent target tidak ditemukan                      |
| `409` | `INVALID_MOVE`     | Move akan membuat cycle                            |
| `500` | `INTERNAL_ERROR`   | Error internal; detail hanya dicatat di server log |

Semua error menggunakan bentuk response yang konsisten:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "human-readable message"
  }
}
```

## Keputusan arsitektur

### Mengapa raw SQL, bukan ORM?

Relasi menu adalah self-referencing dan operasi move memerlukan recursive CTE,
advisory lock, serta beberapa update terkoordinasi. Raw SQL membuat perilaku
tersebut eksplisit dan menghindari query tersembunyi dari ORM. Semua nilai dari
user tetap dikirim sebagai parameter PostgreSQL.

### Mengapa `depth` disimpan?

Depth dibutuhkan frontend untuk menampilkan level menu dan dapat dibaca langsung
tanpa menghitung ulang seluruh ancestor setiap request. Nilainya dihitung saat
create dan diperbarui untuk seluruh subtree saat move.

### Mengapa `position` 0-based?

Indeks ini langsung cocok dengan array pada frontend dan memudahkan operasi
insert/reorder. Nilai position yang melebihi jumlah sibling tidak menyebabkan
gap; repository meng-clamp ke posisi terakhir.

### Mengapa repository memakai interface di service?

Service dapat diuji menggunakan stub/mock repository. Hal ini menjaga business
rules tetap dapat diverifikasi tanpa koneksi database, sekaligus memisahkan
infrastruktur PostgreSQL dari logic aplikasi.

## Troubleshooting

### `DATABASE_URL is required`

Pastikan file `.env` berada di root project dan memiliki `DATABASE_URL`, atau
set environment variable tersebut secara manual sebelum menjalankan server.

### Server gagal melakukan ping database

Periksa host, port, database, username, password, firewall, dan `sslmode`.
Untuk Neon, gunakan connection string direct dengan `sslmode=require`.

### `relation "menus" does not exist`

Migration belum dijalankan. Jalankan `migrate ... up` sesuai langkah instalasi.

### `migrate: command not found`

Pastikan binary hasil `go install` berada pada `GOPATH/bin` dan folder tersebut
tersedia di `PATH`, lalu buka ulang terminal.

### Swagger tidak mencerminkan perubahan endpoint

Regenerate docs dengan `swag init -g cmd/api/main.go -o docs`, kemudian restart
server.

## Status technical test

### Sudah tersedia

- REST CRUD menu.
- Struktur parent-child dengan unlimited depth.
- PostgreSQL migration dan foreign key cascade.
- Input validation dan format error konsisten.
- Swagger/OpenAPI.
- Move dan reorder sebagai fitur bonus.
- Transaksi atomik untuk operasi perubahan struktur.
- Pencegahan cycle dan sinkronisasi depth subtree.
- Unit test service dasar.

### Catatan scope repository ini

Repository ini berisi backend API. Frontend React/Next.js, Tailwind UI,
drag-and-drop, search/filter, dan screenshot/demo merupakan bagian terpisah dari
deliverable fullstack dan belum berada di folder repository ini. Endpoint telah
disiapkan agar frontend dapat menampilkan tree, mengedit nama, menghapus node,
serta melakukan move/reorder.

## Lisensi

Project ini dibuat untuk kebutuhan technical test by Muhammad Rizky Saputra.
