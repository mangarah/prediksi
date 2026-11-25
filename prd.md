PRD Sistem Harga & Stok Barang Second
24 Nov 2026 · @kazoku
Ringkasan
Sistem ini membantu toko membeli barang second di harga yang aman dan menjualnya di harga yang menguntungkan, berdasarkan data harga nyata, bukan perkiraan. Backend dan web dashboard ditulis full Go, ditambah aplikasi mobile, untuk pemilik, karyawan, dan beberapa cabang.
Masalah hari ini:
• Harga beli dari tiap penjual berbeda-beda, dan harga di toko lain/marketplace juga berbeda. Tanpa catatan, sulit tahu harga wajar sebuah barang.
• Harga jual sering ditentukan dengan feeling, sehingga margin tidak konsisten atau barang terlalu lama tidak laku.
• Tidak ada data barang mana yang paling cepat laku, jadi modal bisa tertahan di barang yang kurang diminati.
Solusinya: setiap harga (dari penjual, toko pesaing, marketplace, dan transaksi toko sendiri) dikumpulkan per model barang dan kondisi. Dari situ sistem menghitung harga pasar, memberi batas harga beli maksimal, merekomendasikan harga jual, memprediksi barang yang paling diminati, dan memperkirakan volume penjualannya.
Fokus awal: konsol PlayStation dan Nintendo, game, dan aksesoris. Struktur kategori dibuat umum agar bisa dipakai untuk HP atau barang second lain.
Tujuan dan metrik sukses
Target utama: setiap barang dibeli di bawah harga pasar dan terjual dengan margin yang direncanakan, dalam waktu yang wajar.
Tujuan
Metrik
Target awal (bisa diubah)
Harga beli aman
% pembelian di bawah batas harga beli maksimal
≥ 90%
Margin konsisten
Rata-rata margin kotor per barang
sesuai target per kategori (konsol 12%, game 25%, aksesoris 30%)
Perputaran stok cepat
Rata-rata hari barang di gudang sampai terjual
turun 30% dalam 3 bulan
Stok tidak mengendap
Jumlah barang > 30 hari belum terjual
< 10% dari stok
Prediksi berguna
% barang "diminati tinggi" yang terjual dalam 14 hari
≥ 70%
Data lengkap
% barang yang punya ≥ 5 data harga pembanding
≥ 80% model yang aktif
Bukan tujuan (di luar cakupan v1):
• Toko online publik / e-commerce untuk pembeli. Sistem ini alat internal toko.
• Pembayaran online dan integrasi payment gateway.
• Akuntansi lengkap (pajak, neraca). Cukup laporan laba per barang dan per periode.
• Machine learning yang rumit di awal. Prediksi v1 memakai statistik sederhana yang bisa dijelaskan.
Pengguna dan hak akses
Ada empat peran. Setiap user terikat ke satu atau beberapa cabang, kecuali Owner yang melihat semua cabang.
Peran
Platform utama
Boleh melakukan
Tidak boleh
Owner
Web
Semua: atur cabang, user, target margin, lihat laba semua cabang, setujui harga khusus
—
Manajer cabang
Web + mobile
Kelola stok & harga cabangnya, setujui pembelian di atas batas, lihat laporan cabang
Lihat laba cabang lain, ubah pengaturan global
Buyer / staf pembelian
Mobile
Cek harga pasar, catat pembelian + foto, input harga toko pesaing
Ubah harga jual, lihat laba total
Kasir / staf penjualan
Web + mobile
Catat penjualan, cek stok antar cabang
Ubah harga di bawah harga minimal tanpa persetujuan
Alur utama per peran:
1.  Buyer menerima tawaran barang → scan/pilih model di aplikasi → lihat harga pasar dan batas beli maksimal → tawar → catat pembelian dengan foto dan kondisi.
2.  Sistem menghitung rekomendasi harga jual → barang masuk stok cabang.
3.  Kasir menjual barang → laba tercatat otomatis.
4.  Owner/manajer melihat dashboard: stok mengendap, barang paling diminati, saran barang untuk dibeli, dan notifikasi.
Fitur dan kebutuhan fungsional
Fitur dibagi ke 12 modul. Kolom Fase menunjukkan kapan dibangun (detail di bagian Roadmap).
1. Katalog barang
Fitur
Detail
Fase
Kategori & model
Kategori → merek → model → varian (misal PS5 → Disc / Digital / Slim, 825GB / 1TB)
MVP
Grade kondisi standar
A (mulus), B (lecet ringan), C (lecet/ada minus), D (rusak/untuk part). Kriteria tertulis per kategori
MVP
Kelengkapan
Checklist: dus, stik, kabel, charger, nota, garansi. Masing-masing bisa memengaruhi harga
MVP
Atribut khusus
Misal versi firmware, region, status jailbreak/patch, kapasitas
Fase 2
2. Data harga pembanding
Fitur
Detail
Fase
Input manual
Catat harga tawaran penjual dan harga di toko lain: model, grade, kelengkapan, harga, sumber, tanggal
MVP
Riwayat transaksi sendiri
Setiap pembelian dan penjualan otomatis menjadi data harga
MVP
Daftar toko pesaing
Simpan toko pesaing (nama, link, kota); input harga rutin atau dipantau otomatis
MVP (manual), Fase 2 (otomatis)
Scraping marketplace
Ambil harga listing dari marketplace untuk model yang dipantau, jadwal harian
Fase 2
Pembersihan data
Buang harga tidak wajar (outlier), tandai listing duplikat, deteksi listing "rusak"/"minus" dari judul
Fase 2
3. Pembelian barang (dari orang)
Fitur
Detail
Fase
Cek harga cepat
Pilih model + grade + kelengkapan → tampil harga pasar, batas beli maksimal, dan estimasi laba
MVP
Catat pembelian
Data penjual (nama, no. HP), foto barang, nomor seri/IMEI, harga deal, metode bayar
MVP
Persetujuan
Pembelian di atas batas beli maksimal butuh persetujuan manajer/owner lewat aplikasi
MVP
Cek nomor seri ganda
Peringatan jika nomor seri sudah pernah masuk sistem (mencegah barang curian/penipuan berulang)
MVP
Daftar hitam penjual
Tandai penjual bermasalah
Fase 2
4. Stok dan inventori
Fitur
Detail
Fase
Stok per unit
Setiap barang second = 1 unit unik dengan ID/barcode, status: dicek → servis → siap jual → terjual / retur
MVP
Biaya tambahan
Biaya servis, cleaning, ganti part ditambahkan ke modal unit
MVP
Umur stok
Hitung hari sejak dibeli; tandai > 14, > 30, > 60 hari
MVP
Multi-cabang
Stok per cabang, transfer antar cabang dengan status kirim/terima
MVP
Cetak label
Label barcode/QR dengan harga untuk ditempel
Fase 2
Stok opname
Hitung fisik vs sistem lewat scan mobile
Fase 2
5. Harga jual
Fitur
Detail
Fase
Rekomendasi harga jual
Dihitung dari harga pasar + target margin (rumus di bagian Logika harga)
MVP
Harga minimal
Kasir tidak bisa jual di bawah harga minimal tanpa persetujuan
MVP
Saran turun harga
Barang mengendap → saran turun harga bertahap, tidak di bawah modal kecuali disetujui
Fase 2
6. Penjualan dan laporan laba
Fitur
Detail
Fase
Catat penjualan
Scan/pilih unit, harga jual, diskon, metode bayar, data pembeli (opsional), garansi toko
MVP
Nota
Nota digital (PDF / kirim WhatsApp)
MVP
Laba per unit
Harga jual − (harga beli + biaya tambahan)
MVP
Laporan
Laba per hari/bulan, per cabang, per kategori, per staf; nilai stok; barang mengendap
MVP
Retur & garansi
Catat klaim garansi, barang kembali ke stok
Fase 2
Ekspor
Ekspor laporan ke Excel/CSV
Fase 2
7. Prediksi minat dan saran pembelian
Fitur
Detail
Fase
Ranking barang terlaris
Dari penjualan toko: jumlah terjual dan rata-rata hari sampai laku
MVP
Skor permintaan
Gabungan data toko + tren marketplace (rumus di bagian Prediksi)
Fase 2
Prediksi volume penjualan
Perkiraan unit terjual per model per cabang, 4 minggu ke depan, dengan rentang
Fase 2 (sederhana), Fase 3 (musiman + event)
Saran barang untuk dibeli
Daftar model yang sebaiknya dicari, jumlah, dan batas harga belinya
Fase 2
Prediksi harga ke depan
Tren harga naik/turun per model (misal menjelang rilis konsol baru)
Fase 3
8. Notifikasi
Fitur
Detail
Fase
Kanal
Telegram bot (mudah dan gratis) di awal; WhatsApp di fase berikutnya
MVP (Telegram)
Jenis peringatan
Permintaan persetujuan pembelian, stok > 30 hari, nomor seri ganda, ringkasan harian penjualan & laba
MVP
Peluang beli
Listing marketplace di bawah batas beli maksimal untuk model yang diminati
Fase 2
Perubahan harga pasar
Harga pasar suatu model turun/naik > 10% dalam 7 hari
Fase 2
9. Integrasi toko online
Sistem ini kemungkinan disambungkan ke toko online, jadi sejak awal semua data diakses lewat API yang rapi. Integrasinya sendiri dibangun belakangan.
Fitur
Detail
Fase
API terpisah untuk integrasi
Endpoint /api/integration/v1 dengan API key per toko online; hanya baca stok siap jual, foto, dan harga jual
Fase 3
Sinkron stok
Unit yang terjual di toko fisik langsung hilang dari toko online, dan sebaliknya (unit dikunci saat ada pesanan)
Fase 3
Webhook
Kirim event unit.ready, unit.sold, price.changed ke toko online
Fase 3
Pesanan online
Pesanan dari toko online tercatat sebagai penjualan dengan kanal "online", masuk laporan laba
Fase 3
10. Tukar tambah
Tukar tambah dicatat sebagai satu transaksi berisi dua sisi: unit yang masuk dari pelanggan dan unit yang keluar dari toko. Pelanggan hanya membayar selisihnya, tapi laba tetap dihitung benar per unit.
Fitur
Detail
Fase
Nilai tukar
Barang pelanggan dinilai dengan cek harga yang sama seperti pembelian; nilai tukar ≤ batas beli maksimal, di atas itu butuh persetujuan
MVP
Satu transaksi
Unit masuk (modal = nilai tukar) + unit keluar (dijual di harga penuh) + selisih yang dibayar
MVP
Selisih terbalik
Jika nilai tukar lebih besar dari harga barang yang diambil, toko membayar sisanya dan tercatat sebagai kas keluar
MVP
Laporan
Laba unit keluar = harga jual penuh − modal; unit masuk mulai dihitung umur stoknya
MVP
11. Kas harian per cabang
Setiap cabang membuka dan menutup sesi kas setiap hari, sehingga selisih uang langsung ketahuan.
Fitur
Detail
Fase
Buka kas
Saldo awal tunai dicatat saat toko buka
MVP
Arus kas otomatis
Penjualan, selisih tukar tambah, pembayaran ke penjual, dan biaya servis otomatis masuk kas sesuai metode bayar
MVP
Pengeluaran lain
Catat biaya operasional (bensin, makan, ongkir) dengan foto bukti
MVP
Non-tunai
Transfer dan QRIS dicatat per rekening, untuk dicocokkan manual dengan mutasi bank
MVP
Tutup kas
Hitung fisik vs saldo sistem; selisih > Rp10.000 butuh persetujuan manajer; kas yang ditutup tidak bisa diubah, koreksi lewat entri baru
MVP
Rekap
Rekap kas harian per cabang dan semua cabang, dikirim di ringkasan Telegram
MVP
12. Pengaturan
Semua angka yang dipakai rumus bisa diubah Owner dari satu halaman, dan setiap perubahan tercatat di audit log. Sebagian bisa dibedakan per cabang.
Kelompok
Yang diatur
Per cabang?
Harga
Target margin & margin minimal per kategori, periode data (60 hari), umur paruh bobot (30 hari), bobot per sumber, faktor listing → harga deal (0,93)
Tidak
Kondisi
Faktor grade, potongan per item kelengkapan, kriteria & foto contoh grade
Tidak
Persetujuan
Batas yang butuh persetujuan (di atas batas beli, di bawah harga minimal, selisih kas)
Ya
Stok
Batas umur stok (14 / 30 / 60 hari), saran turun harga bertahap
Ya
Prediksi
Bobot skor permintaan, horizon saran beli (2 minggu), event manual
Tidak
Garansi & penjual
Lama garansi per kategori, aturan simpan KTP penjual
Tidak
Operasional
Metode bayar & rekening, jam ringkasan Telegram, jadwal scraping
Ya
Organisasi
Cabang, user, peran
—
Logika harga
Harga pasar memakai median berbobot, bukan rata-rata biasa, supaya satu harga yang terlalu murah atau terlalu mahal tidak merusak hasil. Semua angka di bawah adalah nilai awal dan bisa diubah Owner di pengaturan.
Langkah 1 — Kumpulkan data pembanding. Ambil semua harga untuk model + varian + grade yang sama dalam 60 hari terakhir. Harga beli (tawaran penjual) dan harga jual (toko, pesaing, marketplace) dihitung sebagai dua seri terpisah.
Langkah 2 — Buang outlier. Harga di luar rentang Q1 − 1,5×IQR sampai Q3 + 1,5×IQR dibuang dari perhitungan, tapi tetap disimpan.
Langkah 3 — Beri bobot berdasarkan sumber dan umur data (data 30 hari lalu bernilai setengah data hari ini):
w = w_{sumber} \times 0.5^{\,umur\_hari / 30}
Sumber
Bobot awal
Catatan
Penjualan toko sendiri
1,0
Harga yang benar-benar terjadi
Toko pesaing (input manual)
0,8
Harga pajang, belum tentu laku
Listing marketplace
0,6
Harga diminta; dikali 0,93 sebagai perkiraan harga deal
Tawaran penjual ke toko
1,0
Hanya untuk seri harga beli
Langkah 4 — Sesuaikan grade dan kelengkapan jika data untuk grade tersebut kurang dari 5. Faktor grade awal: A = 1,00; B = 0,90; C = 0,78; D = 0,50. Kelengkapan memakai potongan nominal per item (misal tanpa dus −Rp150.000, stik kurang satu −Rp400.000).
Langkah 5 — Hitung tiga angka utama:
Harga\ pasar\ jual\ (P) = median\ berbobot(harga\ jual)
Batas\ beli\ maksimal = P \times (1 - margin_{target}) - biaya_{estimasi}
Harga\ jual\ rekomendasi = \max(P,\ modal \times (1 + margin_{min}))
Harga jual dibulatkan ke atas per Rp10.000. Harga minimal (batas kasir) = modal × (1 + margin minimal), default margin minimal 5%.
Langkah 6 — Tingkat keyakinan. Setiap angka ditampilkan dengan jumlah data dan rentang harga (P25–P75). Kurang dari 5 data = label "Data kurang", dan pembelian butuh persetujuan manajer.
Contoh: PS5 Slim Disc, grade A, lengkap. Harga pasar jual P = Rp7.200.000 dari 14 data, target margin 15%, estimasi biaya cleaning Rp100.000.
Angka
Hitungan
Hasil
Batas beli maksimal
7.200.000 × 0,85 − 100.000
Rp6.020.000
Jika dibeli di
—
Rp5.800.000
Modal (beli + biaya)
5.800.000 + 100.000
Rp5.900.000
Harga jual rekomendasi
max(7.200.000; 5.900.000 × 1,05)
Rp7.200.000
Harga minimal kasir
5.900.000 × 1,05
Rp6.195.000 → Rp6.200.000
Estimasi laba
7.200.000 − 5.900.000
Rp1.300.000 (18%)
Setiap perhitungan menyimpan snapshot (data mana yang dipakai), agar Owner bisa melihat alasan di balik angka.
Prediksi minat dan saran pembelian
Setiap model mendapat Skor Permintaan 0–100 setiap malam, dan skor itu yang menentukan barang apa yang disarankan untuk dibeli. Versi pertama memakai statistik sederhana agar alasannya bisa dijelaskan; model prediksi yang lebih canggih baru di Fase 3 setelah data cukup (minimal 3–6 bulan transaksi).
Komponen skor (setiap komponen diubah ke persentil 0–100 dibanding semua model lain, lalu dijumlah dengan bobot):
Komponen
Sumber data
Bobot awal
Kecepatan laku: median hari dari siap jual sampai terjual (makin cepat makin tinggi)
Penjualan toko
35%
Volume: jumlah terjual 30 hari terakhir, semua cabang
Penjualan toko
25%
Tren marketplace: listing yang hilang dalam 7 hari (perkiraan terjual) dibanding listing baru
Scraping marketplace
25%
Arah harga: perubahan harga pasar 30 hari (naik = diminati)
Semua sumber harga
15%
Sebelum scraping aktif (MVP), skor hanya memakai dua komponen dari penjualan toko, dengan bobot dinormalkan ulang.
Kategori hasil: Skor ≥ 70 = Diminati tinggi; 40–69 = Normal; < 40 = Lambat (hindari stok banyak, beli hanya di harga sangat murah).
Saran barang untuk dibeli (dihitung per cabang):
Kebutuhan = prediksi\ volume\ 2\ minggu - stok\ siap\ jual
Sebelum prediksi volume aktif, prediksi volume diganti rata-rata terjual per minggu × 2.
Model dengan Skor ≥ 70 dan Kebutuhan > 0 muncul di daftar "Cari barang ini", lengkap dengan jumlah dan batas beli maksimal dari bagian Logika harga. Daftar ini juga dikirim ke buyer lewat Telegram setiap pagi.
Prediksi volume penjualan
Selain skor minat, sistem memperkirakan berapa unit tiap model akan terjual per minggu, per cabang, untuk 4 minggu ke depan. Hasilnya ditampilkan sebagai angka tengah plus rentang (misal: PS5 Slim, cabang A, minggu depan: 6 unit, rentang 4–9).
Tahap
Syarat data
Metode
Fase
Awal
≥ 4 minggu penjualan
Rata-rata bergerak 4 minggu, disesuaikan dengan arah Skor Permintaan
Fase 2
Musiman
≥ 8–12 minggu
Ditambah pola mingguan/bulanan (akhir pekan, tanggal gajian)
Fase 2
Lanjut
≥ 6 bulan
Holt-Winters (exponential smoothing dengan tren dan musiman) ditulis di Go, ditambah faktor event manual: rilis konsol/game baru, Lebaran, promo
Fase 3
• Model yang datanya masih sedikit diprediksi di level kategori atau merek, lalu dibagi ke model sesuai porsi penjualannya.
• Prediksi volume dipakai langsung untuk saran pembelian (rumus Kebutuhan di atas) dan untuk dashboard "Perkiraan penjualan 4 minggu" per cabang.
• Akurasi dicek tiap minggu: prediksi dibanding penjualan nyata, ditampilkan di dashboard. Target awal: selisih rata-rata < 30% untuk model yang laku ≥ 2 unit per minggu.
• Prediksi arah harga per model (naik/turun) tetap di Fase 3, setelah data harga cukup panjang.
Catatan: "listing hilang" belum tentu terjual (bisa dihapus penjual). Karena itu bobotnya lebih kecil daripada data penjualan sendiri, dan akurasinya dievaluasi tiap bulan dengan membandingkan skor dengan penjualan nyata.
Arsitektur teknis
Backend dan web dibuat full Go sebagai modular monolith: satu aplikasi yang melayani API dan halaman web, satu worker untuk pekerjaan terjadwal (scraping, hitung skor, notifikasi), dan satu database PostgreSQL. Ini cukup untuk satu developer dan multi-cabang, dan modul bisa dipecah jadi service terpisah nanti jika perlu.
Web dan mobile hanya bicara ke API; pekerjaan berat dan terjadwal dijalankan worker, sehingga cek harga di toko tetap cepat.
Stack
Lapisan
Pilihan
Alasan
Bahasa backend
Go 1.23+
Sesuai rencana; cepat, satu binary, mudah deploy
HTTP router
chi
Ringan, dekat dengan net/http standar
Database
PostgreSQL 16
Transaksi kuat untuk stok & uang, fungsi percentile untuk median
Akses data
pgx + sqlc
SQL ditulis manual, kode Go dibuat otomatis dan type-safe
Migrasi
goose
Sederhana, berbasis file SQL
Job queue & jadwal
River (antrean di PostgreSQL)
Tidak perlu Redis; retry otomatis
Scraper
colly (halaman statis), go-rod (halaman berbasis JavaScript)
Keduanya di Go
Penyimpanan foto
Object storage S3-compatible (Cloudflare R2 atau MinIO)
Foto barang tidak membebani database
Auth
JWT access token pendek + refresh token, RBAC per peran & cabang
Satu mekanisme untuk web dan mobile
Web dashboard
Go templ + htmx (+ Alpine.js untuk interaksi kecil)
Satu bahasa untuk semua; halaman dirender di server, tanpa build JavaScript terpisah
Aplikasi mobile
Flutter
Satu kode untuk Android & iOS, kamera & scan barcode
Notifikasi
Telegram Bot API; WhatsApp Business API di fase lanjut
Telegram gratis dan cepat dibuat
Deploy
Docker Compose di VPS, backup database harian
Murah untuk awal
Struktur modul Go
cmd/server         -> HTTP server: API JSON + halaman web
cmd/worker         -> job terjadwal (scrape, skor, prediksi, notifikasi)
internal/web       -> handler & komponen templ untuk dashboard
internal/catalog   -> kategori, model, grade, kelengkapan
internal/pricing   -> data harga, outlier, median berbobot, rekomendasi
internal/purchase  -> pembelian, penjual, persetujuan
internal/inventory -> unit, biaya, transfer, umur stok
internal/sales     -> penjualan, nota, laba
internal/demand    -> skor permintaan, prediksi volume, saran beli
internal/scraper   -> target scrape, parser per marketplace
internal/integration -> API key, webhook untuk toko online
internal/notify    -> Telegram, WhatsApp
internal/auth      -> user, peran, cabang, audit log
Data model inti
Tabel
Isi utama
branches, users, user_branches
Cabang, user, peran per cabang
categories, brands, product_models, variants
Katalog barang
grades, accessories
Definisi grade dan item kelengkapan
price_observations
Setiap data harga: sumber, model, varian, grade, kelengkapan (jsonb), harga, tanggal, url, cabang, is_outlier
competitors, scrape_targets, scrape_runs, marketplace_listings
Toko pesaing, jadwal & hasil scraping
sellers
Penjual barang ke toko, termasuk status daftar hitam
purchases, approvals
Pembelian, persetujuan di atas batas
trade_ins
Transaksi tukar tambah: purchase_id (unit masuk), sale_id (unit keluar), selisih, arah bayar
units, unit_costs, unit_events
Unit stok unik (nomor seri, status, modal), biaya servis, riwayat status
transfers
Perpindahan unit antar cabang
sales, sale_items, warranties
Penjualan, nota, garansi
cash_sessions, cash_movements, payment_accounts
Sesi kas harian per cabang, setiap uang masuk/keluar, rekening & metode bayar
price_recommendations
Snapshot hasil hitung harga + data yang dipakai
demand_scores, sales_forecasts
Skor permintaan harian dan prediksi volume per model per cabang
settings, setting_overrides
Pengaturan global dan pengecualian per cabang
imports
Riwayat impor Excel/CSV beserta error per baris
notifications, audit_logs
Riwayat notifikasi dan jejak perubahan
Uang disimpan sebagai bigint (rupiah utuh), bukan float. Perubahan stok, pembelian, dan penjualan selalu dalam satu transaksi database agar satu unit tidak bisa terjual dua kali.
API utama (REST, /api/v1)
Endpoint
Fungsi
GET /pricing/quote?model&variant&grade&accessories
Harga pasar, batas beli, harga jual rekomendasi, jumlah data
POST /price-observations
Input harga manual (penjual/pesaing)
POST /purchases
Catat pembelian, otomatis membuat unit
POST /trade-ins
Catat tukar tambah: unit masuk, unit keluar, selisih
POST /approvals/{id}/approve
Setujui pembelian/harga khusus/selisih kas
GET /units?status&branch&age_gt
Daftar stok dan filter umur
POST /transfers
Pindah unit antar cabang
POST /sales
Catat penjualan, cek harga minimal
POST /cash-sessions, POST /cash-sessions/{id}/close
Buka dan tutup kas harian
POST /cash-movements
Pengeluaran/pemasukan lain di luar transaksi
GET /reports/profit?from&to&branch
Laporan laba
GET /demand/top?branch
Ranking skor permintaan
GET /demand/forecast?branch&weeks=4
Prediksi volume penjualan
GET /demand/buy-list?branch
Saran barang untuk dibeli
GET, PUT /settings
Baca dan ubah pengaturan
POST /imports/{catalog,units,prices}
Impor data awal dari Excel/CSV dengan pratinjau
POST /sync
Kirim antrean transaksi offline dari mobile (idempoten)
Daftar halaman
Web dipakai untuk mengelola dan melihat laporan; mobile dipakai di lapangan dan di meja kasir. Kolom Peran menunjukkan siapa yang bisa membuka halaman itu.
Web (Go templ + htmx)
Halaman
Isi utama
Peran
Fase
Login
Email/no. HP + password
Semua
MVP
Dashboard
Laba & penjualan hari ini, persetujuan menunggu, stok > 30 hari, 10 barang paling diminati, kas cabang
Owner, Manajer
MVP
Cek harga
Pilih model/varian/grade/kelengkapan → harga pasar, batas beli, harga jual, rentang, jumlah data, daftar data pembanding
Semua
MVP
Pembelian
Daftar pembelian + form catat pembelian
Owner, Manajer, Buyer
MVP
Tukar tambah
Form dua sisi: barang masuk dan barang keluar, selisih otomatis
Owner, Manajer, Kasir
MVP
Kasir / Penjualan
Scan barcode unit, harga, diskon, metode bayar, cetak/kirim nota
Owner, Manajer, Kasir
MVP
Stok
Daftar unit dengan filter status, cabang, umur; detail unit dengan foto, modal, riwayat
Semua (laba disembunyikan untuk Kasir & Buyer)
MVP
Transfer
Kirim dan terima unit antar cabang
Owner, Manajer
MVP
Kas harian
Buka/tutup kas, arus kas hari ini, pengeluaran lain
Owner, Manajer, Kasir
MVP
Data harga
Input harga manual, daftar toko pesaing, hasil scraping
Owner, Manajer, Buyer
MVP (manual), Fase 2
Prediksi & saran beli
Skor permintaan, prediksi volume 4 minggu, daftar "Cari barang ini"
Owner, Manajer
MVP (ranking), Fase 2
Laporan
Laba, nilai stok, stok mengendap, kas, kinerja staf; ekspor
Owner, Manajer
MVP
Persetujuan
Antrean pembelian, harga, dan selisih kas yang menunggu
Owner, Manajer
MVP
Katalog
Kategori, merek, model, varian, grade, kelengkapan; impor Excel
Owner
MVP
Penjual
Data penjual, riwayat jual ke toko, daftar hitam
Owner, Manajer
MVP
Pengaturan
Semua kelompok pengaturan (modul 12)
Owner
MVP
User & cabang
Tambah/nonaktifkan user, peran, akses cabang
Owner
MVP
Audit log
Siapa mengubah apa dan kapan
Owner
MVP
Mobile (Flutter)
Layar
Isi utama
Offline?
Fase
Login
Login sekali, sesi panjang dengan PIN
—
MVP
Beranda
Tugas hari ini, persetujuan (manajer), daftar "Cari barang ini"
Baca cache
MVP
Cek harga
Cari model atau scan barcode dus, pilih grade & kelengkapan
Ya (harga terakhir)
MVP
Catat pembelian
Foto barang, scan nomor seri, data penjual, foto KTP, harga deal
Ya
MVP
Tukar tambah
Sama seperti web, versi ringkas
Tidak
MVP
Cek stok
Scan barcode unit → status, harga, cabang
Tidak
MVP
Jual cepat
Penjualan satu unit dari HP
Tidak
MVP
Input harga pesaing
Model, grade, harga, foto/link sumber
Ya
MVP
Persetujuan
Setujui/tolak dengan alasan
Tidak
MVP
Antrean offline
Daftar data yang belum terkirim dan statusnya
—
MVP
Kriteria selesai (MVP)
Sebuah fitur MVP dianggap selesai hanya jika semua kriteria di bawah lolos tes otomatis atau tes manual di staging. Centang saat lolos. Kriteria Fase 2 dan 3 ditulis sebelum fase itu dimulai.
Harga
[ ] Cek harga mengembalikan harga pasar, batas beli maksimal, harga jual rekomendasi, rentang P25–P75, dan jumlah data dalam < 500 ms.
[ ] Contoh PS5 Slim di bagian Logika harga menghasilkan angka yang sama persis (jadi test case).
[ ] Data outlier tidak ikut dihitung, tapi tetap tersimpan dan terlihat di daftar pembanding.
[ ] Model dengan < 5 data berlabel "Data kurang", dan pembeliannya otomatis butuh persetujuan.
Pembelian dan tukar tambah
[ ] Menyimpan pembelian otomatis membuat unit, data harga, dan kas keluar (jika tunai) dalam satu transaksi; jika salah satu gagal, semuanya batal.
[ ] Pembelian di atas batas beli berstatus "Menunggu persetujuan", dan unitnya tidak bisa dijual sebelum disetujui.
[ ] Nomor seri yang sudah pernah masuk memunculkan peringatan dan butuh alasan untuk lanjut.
[ ] Tukar tambah menghasilkan satu unit masuk, satu unit keluar, dan satu baris kas sebesar selisih saja.
[ ] Laba unit keluar pada tukar tambah dihitung dari harga jual penuh, bukan dari selisih.
Stok dan penjualan
[ ] Dua kasir yang menjual unit yang sama bersamaan: hanya satu berhasil, yang lain mendapat pesan "unit sudah terjual".
[ ] Unit berstatus "Dalam pengiriman" tidak bisa dijual di cabang mana pun.
[ ] Penjualan di bawah harga minimal ditolak kecuali disetujui manajer.
[ ] Laba per unit = harga jual − (harga beli + semua biaya tambahan).
[ ] Nota PDF bisa diunduh dan dibagikan ke WhatsApp.
Kas harian
[ ] Saldo sistem = saldo awal + uang masuk tunai − uang keluar tunai, cocok dengan semua transaksi hari itu.
[ ] Tutup kas dengan selisih > Rp10.000 butuh persetujuan manajer.
[ ] Kas yang sudah ditutup tidak bisa diubah; koreksi hanya lewat entri baru yang tercatat di audit log.
Akses, pengaturan, notifikasi
[ ] User cabang A tidak bisa membaca data cabang B, termasuk saat memanggil API langsung.
[ ] Kasir dan Buyer tidak melihat modal dan laba.
[ ] Mengubah pengaturan langsung memengaruhi perhitungan berikutnya dan tercatat di audit log (nilai lama dan baru).
[ ] Permintaan persetujuan sampai ke Telegram < 1 menit; ringkasan harian terkirim di jam yang diatur.
Mobile dan data awal
[ ] Pembelian yang dibuat saat offline terkirim otomatis saat online, tanpa duplikat walau dikirim ulang.
[ ] Impor katalog dari Excel menampilkan pratinjau dan error per baris sebelum disimpan.
Data awal dan mode offline
Isi data awal (go-live)
Sistem baru berguna setelah katalog, stok, dan sedikit data harga terisi. Semua diisi lewat impor Excel/CSV dengan template yang bisa diunduh dari sistem.
Data
Cara isi
Catatan
Katalog (kategori, merek, model, varian)
Template Excel, disusun sekali untuk PS dan Nintendo
Bisa dibantu AI untuk menyusun daftar model, lalu dicek manual
Grade & kelengkapan
Otomatis dari nilai default, lalu disesuaikan
Kriteria grade + foto contoh
Stok yang ada sekarang
Impor Excel: model, grade, nomor seri, modal, tanggal beli (perkiraan boleh)
Diikuti stok opname fisik
Harga historis
Impor dari catatan lama jika ada
Masuk sebagai sumber "riwayat toko"
Cabang, user, rekening
Diisi manual di halaman User & cabang dan Pengaturan
—
Impor selalu lewat pratinjau: baris yang salah ditandai dengan alasannya, baris yang benar baru disimpan setelah dikonfirmasi.
Urutan go-live:
1.  Isi katalog dan pengaturan.
2.  Impor stok sekarang, lalu stok opname fisik per cabang.
3.  Input minimal 5 harga pembanding untuk 30 model paling sering dijual.
4.  Pakai paralel dengan catatan lama selama 1 minggu di 1 cabang.
5.  Jika angka laba dan kas cocok, catatan lama dihentikan (Gate 1).
Mode offline di mobile
Yang bisa dilakukan offline hanya pekerjaan lapangan yang tidak mengubah stok yang sedang dijual. Transaksi yang butuh status stok real-time wajib online.
Bisa offline
Wajib online
Cek harga (dari harga terakhir yang tersimpan)
Penjualan dan jual cepat
Catat pembelian + foto
Tukar tambah
Input harga pesaing
Transfer stok
Lihat daftar "Cari barang ini"
Persetujuan, buka/tutup kas
Aturan sinkron:
• Setiap transaksi offline mendapat ID unik dari HP (UUID v7). Server menolak ID yang sudah pernah diterima, jadi kirim ulang tidak membuat data ganda.
• Harga di cache diberi tanggal; lebih dari 24 jam muncul peringatan "harga mungkin sudah berubah".
• Jika saat sinkron nomor seri ternyata ganda, atau harga deal melewati batas beli terbaru, pembelian tetap tersimpan dengan status "Perlu dicek" dan masuk antrean persetujuan manajer.
• Foto dikirim terpisah setelah data, dengan kompresi, dan dicoba ulang otomatis jika gagal.
Testing, deploy, dan biaya
Testing
Jenis tes
Yang dites
Alat
Unit test
Rumus harga, outlier, skor permintaan, prediksi volume, laba, saldo kas; contoh angka di PRD dijadikan test case
Go testing (table-driven)
Integration test
Query database, transaksi pembelian/penjualan/tukar tambah, hak akses per cabang
testcontainers-go dengan PostgreSQL asli
Tes bersamaan
Dua penjualan untuk unit yang sama di waktu yang sama
Go test dengan goroutine
Tes scraper
Parser per marketplace memakai file HTML yang disimpan
Go testing
End-to-end web
Alur kritis: beli → jual, tukar tambah, tutup kas
Playwright
Mobile
Form pembelian, antrean offline, sinkron ulang
Flutter widget & integration test
Setiap push ke GitHub menjalankan lint (golangci-lint), semua tes, lalu build image Docker lewat GitHub Actions. Kode yang tesnya gagal tidak bisa di-merge.
Deploy
Lingkungan
Fungsi
Data
Lokal
Development di laptop dengan Docker Compose
Data contoh
Staging
Subdomain terpisah untuk uji coba sebelum rilis dan uji kriteria selesai
Data contoh + salinan anonim
Produksi
Dipakai toko
Data asli
• Rilis memakai tag versi (misal v1.2.0); server menarik image baru, migrasi goose dijalankan otomatis sebelum aplikasi menyala.
• Rollback = jalankan lagi image versi sebelumnya; migrasi dibuat agar tetap aman untuk versi lama (tambah kolom dulu, hapus belakangan).
• Backup PostgreSQL harian ke object storage dan salinan kedua ke NAS di rumah.
• Aplikasi Android untuk staf dibagikan sebagai file APK langsung, tanpa Play Store, karena hanya dipakai internal.
Perkiraan biaya
Item
Perkiraan
Catatan
VPS (2–4 vCPU, 8 GB RAM)
VPS KVM 2 yang sudah ada
Cukup untuk awal sampai 100 user; staging bisa di VPS yang sama
Penyimpanan foto (Cloudflare R2)
Gratis sampai 10 GB/bulan
Di atas itu sekitar US$0,015 per GB per bulan
Domain
Sekitar Rp150.000–250.000 per tahun (perkiraan)
Satu domain untuk produksi dan staging
Telegram bot
Gratis
—
Google Play
US$25 sekali bayar, opsional
Tidak perlu jika APK dibagikan langsung
Apple Developer Program
US$99 per tahun
Hanya jika staf memakai iPhone
WhatsApp Business API
Bayar per percakapan sesuai tarif Meta
Fase 3, tidak wajib
Sumber biaya: Cloudflare R2 pricing, Apple Developer Program.
Kebutuhan non-fungsional dan risiko
Kebutuhan non-fungsional
Aspek
Kebutuhan
Kecepatan
Cek harga (quote) < 500 ms; halaman dashboard < 2 detik
Mobile di lapangan
Catat pembelian tetap bisa saat sinyal jelek: simpan offline, kirim saat online
Keamanan
HTTPS, password di-hash (argon2id/bcrypt), rate limit login, hak akses dicek di server per cabang
Data pribadi
No. HP dan KTP penjual hanya terlihat Owner/manajer; foto KTP (jika disimpan) dienkripsi; patuhi UU PDP
Jejak audit
Setiap ubah harga, hapus data, dan persetujuan tercatat siapa, kapan, nilai lama/baru
Integritas stok
Satu unit tidak bisa terjual dua kali (row lock di transaksi)
Backup
Backup PostgreSQL harian, simpan 30 hari, uji restore sebulan sekali
Monitoring
Log terstruktur (slog), notifikasi Telegram jika scraping gagal atau API error
Skala: dirancang untuk sampai 100 user aktif dan jumlah cabang bebas (jumlahnya belum pasti), tanpa perlu ubah arsitektur. Untuk skala ini satu VPS 2–4 vCPU dengan 8 GB RAM cukup; cabang hanyalah data, bukan server terpisah.
Risiko
Risiko
Dampak
Penanganan
Scraping melanggar aturan marketplace atau diblokir
Data tren berhenti, akun/IP diblokir
Cek syarat layanan tiap marketplace; scraping pelan & terbatas pada model yang dipantau; pakai API resmi jika ada; sistem tetap jalan dengan data manual
Struktur halaman marketplace berubah
Parser rusak
Parser per marketplace terpisah, tes harian, peringatan jika hasil 0
Data harga awal sedikit
Rekomendasi kurang akurat
Label "Data kurang", wajib persetujuan; minggu awal fokus input manual
Barang curian dijual ke toko
Masalah hukum, kerugian
Wajib foto + nomor seri, cek nomor seri ganda, simpan identitas penjual
Grade dinilai beda tiap staf
Harga tidak konsisten
Kriteria grade tertulis + foto contoh per grade di aplikasi
Proyek terlalu besar untuk satu orang
Lama selesai
Rilis bertahap; MVP tanpa scraping & tanpa ML
Roadmap
Fase 1 sudah memberi nilai tanpa scraping: harga pasar dihitung dari input manual dan transaksi toko. Fase berikutnya dimulai hanya setelah gate sebelumnya terpenuhi; durasi adalah perkiraan untuk satu developer.
Keputusan default
Pertanyaan yang sebelumnya terbuka diisi dengan nilai awal berikut. Semuanya bisa diubah Owner di halaman Pengaturan tanpa mengubah kode.
Topik
Nilai default
Alasan
Marketplace dipantau dulu
Tokopedia dan Shopee (halaman publik, tanpa login). Facebook Marketplace dan OLX lewat input manual
Facebook butuh login dan paling cepat memblokir akun
Target margin per kategori
Konsol 12%, game 25%, aksesoris 30%
Konsol harganya tinggi dan bersaing ketat; game dan aksesoris murah tapi marginnya bisa lebih besar
Margin minimal (batas kasir)
5% untuk semua kategori
Mencegah jual rugi tanpa persetujuan
Identitas penjual
Wajib nama, no. HP, dan foto KTP untuk pembelian ≥ Rp1.000.000; di bawah itu cukup nama dan no. HP. Foto KTP dienkripsi dan dihapus otomatis setelah 2 tahun
Melindungi toko dari barang curian, sambil membatasi data pribadi yang disimpan
Garansi toko
Konsol 14 hari, stik/aksesoris elektronik 7 hari, game kaset/kartu 3 hari (cek fungsi)
Cukup untuk menemukan cacat tersembunyi tanpa membebani toko
Sudah diputuskan sebelumnya: web dashboard full Go (templ + htmx); dirancang untuk sampai 100 user dengan jumlah cabang bebas; API integrasi toko online di Fase 3; prediksi volume penjualan di Fase 2 (sederhana) dan Fase 3 (musiman + event); tukar tambah, kas harian, dan pengaturan masuk MVP.