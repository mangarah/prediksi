# Portofolio — Prediksi

Studi kasus visual dan tertulis untuk sistem harga & stok barang second.

**Live: https://mangarah.github.io/prediksi/**

## Isi folder

| Path | Isi |
|---|---|
| `index.html` | Halaman studi kasus visual (satu file, tanpa build) |
| `CASE_STUDY.md` | Versi teks untuk GitHub/recruiter |
| `assets/` | Screenshot aplikasi yang berjalan |
| `README.md` | Panduan ini |
| `.nojekyll` | Menonaktifkan pemrosesan Jekyll di GitHub Pages |

## Pratinjau lokal

Buka `index.html` langsung di browser, atau jalankan server statis dari root repo:

```powershell
python -m http.server 8090 --directory portfolio
```

Lalu buka http://127.0.0.1:8090.

Halaman memakai font Google; tanpa internet, font jatuh ke fallback sistem dan tampilan tetap terbaca.

## Publikasi

### GitHub Pages (otomatis)

Workflow `.github/workflows/pages.yml` menerbitkan folder ini setiap push ke `main`.

1. Push repo ke GitHub.
2. Buka **Settings → Pages → Build and deployment → Source: GitHub Actions**.
3. Push perubahan di `portfolio/` atau jalankan workflow manual.

### Tanpa GitHub Actions

- **Netlify/Vercel:** drag-and-drop folder `portfolio/`.
- **Hosting statis lain:** unggah seluruh isi `portfolio/` sebagai root situs.

## Sebelum publikasi

1. Ganti nama **Kazoku** di footer `index.html` dan `CASE_STUDY.md`.
2. Buka komentar di bagian kontak `index.html`, isi email/GitHub/LinkedIn.
3. Ganti placeholder kontak di `CASE_STUDY.md`.
4. Kalau aplikasi sudah punya URL demo publik, tambahkan tautannya.
5. Ambil ulang screenshot setelah ada transaksi pilot nyata agar angkanya kredibel.

## Catatan

- Halaman memakai data demo; bukan klaim hasil produksi.
- Status yang ditulis adalah **pilot internal**, bukan produksi.
- Tidak ada dependency build; cukup HTML, CSS, dan gambar.
