// Package worker menjalankan tugas terjadwal ringan tanpa antrean terpisah.
package worker

import (
	"context"
	"log"
	"strconv"
	"time"

	"prediksi/internal/notify"
	"prediksi/internal/store"
)

// Run mengirim ringkasan harian pada jam yang ditentukan (default 08:00)
// dan membersihkan sesi login yang sudah kedaluwarsa.
func Run(ctx context.Context, st *store.Store, tg *notify.Telegram, summaryHour int) {
	lastSend := ""
	cleanup := time.Now()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if now.Sub(cleanup) > 6*time.Hour {
				_, _ = st.DB.Exec(`DELETE FROM sessions WHERE expires_at < NOW()`)
				cleanup = now
			}
			today := now.Format("2006-01-02")
			if now.Hour() != summaryHour || lastSend == today {
				continue
			}
			lastSend = today
			k, err := st.KPIs(0)
			if err != nil {
				log.Printf("worker ringkasan: %v", err)
				continue
			}
			pending, _ := st.CountApprovals("menunggu", 0)
			text := "Ringkasan hari ini\n" +
				"Penjualan: " + strconv.Itoa(k.SalesToday) + " unit\n" +
				"Omzet: Rp " + strconv.FormatInt(k.RevenueToday, 10) + "\n" +
				"Laba: Rp " + strconv.FormatInt(k.ProfitToday, 10) + "\n" +
				"Stok siap jual: " + strconv.Itoa(k.ReadyUnits) + " unit\n" +
				"Stok mengendap >30 hari: " + strconv.Itoa(k.StaleUnits) + " unit\n" +
				"Menunggu persetujuan: " + strconv.Itoa(pending)
			if tg.Enabled() {
				if err := tg.Send(ctx, text); err != nil {
					log.Printf("telegram: %v", err)
				}
			} else {
				log.Printf("ringkasan (telegram nonaktif): %s", text)
			}
		}
	}
}
