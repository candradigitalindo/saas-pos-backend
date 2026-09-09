import { useEffect, useState } from 'react'

/**
 * Status koneksi.
 *
 * Dipakai untuk menampilkan FAKTA YANG MENENANGKAN di pojok layar, bukan alarm:
 * "✓ Semua data tersimpan" atau "⏳ 3 transaksi menunggu dikirim". Tidak pernah
 * dialog, tidak pernah merah (ui/01-PRINSIP-DESAIN.md §8).
 */
export function useOnline(): boolean {
  const [online, setOnline] = useState(() => navigator.onLine)

  useEffect(() => {
    const naik = () => setOnline(true)
    const turun = () => setOnline(false)
    window.addEventListener('online', naik)
    window.addEventListener('offline', turun)
    return () => {
      window.removeEventListener('online', naik)
      window.removeEventListener('offline', turun)
    }
  }, [])

  return online
}
