# MikVoc

Panel manajemen MikroTik Hotspot dan PPPoE berbasis web. MikVoc menyediakan pengelolaan multi-router, profile bergaya Mikhmon, pembuatan voucher, pencetakan voucher, template login Hotspot per-router, laporan, RBAC, audit log, backup, dan monitoring operasional.

Developed by **SantaiNetwork**.

[![CI](https://github.com/Santainetwork/mikvoc/actions/workflows/ci.yml/badge.svg)](https://github.com/Santainetwork/mikvoc/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Santainetwork/mikvoc)](https://github.com/Santainetwork/mikvoc/releases)

## Fitur

- Multi-router dengan sesi router aktif per browser.
- Manajemen user, profile, active session, dan voucher Hotspot.
- Hotspot Hosts, IP Bindings, Cookies, Servers, dan Server Profiles bergaya Mikhmon.
- System Log RouterOS dengan filter topic dan pencarian untuk operator/owner.
- Manajemen secret, profile, dan active session PPPoE.
- Generate voucher batch dengan harga jual dan masa aktif profile.
- Empat layout cetak voucher: Classic, Thermal, Grid Sheet, dan Compact.
- Logo, nama brand, DNS, dan template cetak voucher berbeda untuk setiap router.
- Template login Hotspot Modern, Informatif, Minimal, Cafe/Gaming, dan Custom.
- Editor `login.html`, `status.html`, `logout.html`, upload ZIP aset, preview sandbox, dan paket ZIP.
- Role `owner`, `operator`, dan `viewer` dengan audit log.
- Password admin bcrypt dan password router terenkripsi AES-256-GCM.
- CSRF protection, login rate limit, security headers, gzip, dan request ID.
- SQLite backup/restore, health check, metrik Prometheus, scheduler, dan keep-alive router.
- Single binary dengan template, CSS, JavaScript, dan font tertanam.

## Persyaratan

- Go 1.25 atau lebih baru.
- RouterOS API aktif dan dapat dijangkau oleh server MikVoc.
- Node.js/npm hanya diperlukan saat mengubah source Tailwind di `web/src`.

## Instalasi

Unduh binary terbaru dari halaman [Releases](https://github.com/Santainetwork/mikvoc/releases), atau build dari source:

```bash
git clone https://github.com/Santainetwork/mikvoc.git
cd mikvoc
cp .env.example .env
openssl rand -hex 32
```

Masukkan hasil perintah terakhir sebagai `MIKVOC_SECRET` di `.env`, lalu build dan jalankan:

```bash
make build
./mikvoc
```

Cek versi binary:

```bash
./mikvoc --version
```

Buka `http://localhost:8080`.

## Konfigurasi

```env
MIKVOC_SECRET=change-me-to-random-32-byte-hex
MIKVOC_PORT=8080
MIKVOC_DB=mikvoc.db
```

`MIKVOC_SECRET` wajib tetap sama setelah restart. Perubahan secret membuat session lama tidak valid dan password router yang sudah terenkripsi tidak dapat didekripsi.

Nilai juga dapat diberikan lewat flag:

```bash
./mikvoc --port 8082 --db /var/lib/mikvoc/mikvoc.db --secret "your-secret"
```

## Pengembangan

```bash
make dev
make test
make vet
make build
```

Build ulang frontend hanya saat mengubah `web/src`:

```bash
cd web
npm install
npm run build
```

## Template Per-Router

Pilih router aktif, lalu buka **Template Hotspot**. Nama brand, logo, teks logo, DNS, dan varian template disimpan sebagai pengaturan router tersebut. Voucher print dan halaman login menggunakan identitas router aktif dengan fallback ke pengaturan global.

File Hotspot yang berukuran kurang dari 4096 byte dan tidak memakai aset dapat dikirim melalui API RouterOS. Untuk file besar atau template dengan aset, unduh paket ZIP lalu unggah isinya melalui Winbox atau WebFig ke folder Hotspot router.

## Endpoint Operasional

| Endpoint | Keterangan |
| --- | --- |
| `/healthz` | Status aplikasi, database, router, dan uptime |
| `/metrics` | Metrik format Prometheus |
| `/settings/backup` | Unduh backup SQLite |
| `/settings/restore` | Restore database, khusus owner |
| `/settings/audit` | Audit log, operator dan owner |

## Keamanan

- Jangan commit `.env`, database SQLite, backup, atau binary hasil build.
- Gunakan secret acak minimal 32 byte pada produksi.
- Batasi akses RouterOS API hanya dari host MikVoc.
- Jalankan MikVoc di belakang reverse proxy HTTPS untuk akses jaringan publik.
- Ganti kredensial admin setelah instalasi awal.

## Lisensi

MikVoc tersedia di bawah [MIT License](LICENSE).

Copyright (c) 2026 SantaiNetwork.
