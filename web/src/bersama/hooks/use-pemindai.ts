import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Pemindai barcode lewat kamera.
 *
 * Dua jalur, dan urutannya penting untuk ukuran unduhan:
 *
 *   1. `BarcodeDetector` bawaan peramban. Tersedia di Chrome Android — persis
 *      perangkat persona utama kita (ui/README: "HP Android kelas menengah",
 *      "tablet 10 inci di meja kasir"). Nol byte tambahan.
 *   2. Kalau tidak ada (Safari/iOS, sebagian desktop), pustaka WASM ditarik
 *      SAAT DIBUTUHKAN saja. Orang yang tidak pernah memindai tidak ikut
 *      mengunduhnya.
 *
 * Format dibatasi ke yang benar-benar dipakai barang warung: EAN-13 (barcode
 * ritel Indonesia), EAN-8, UPC, Code 128, dan QR.
 */

const FORMAT = [
  'ean_13',
  'ean_8',
  'upc_a',
  'upc_e',
  'code_128',
  'code_39',
  'qr_code',
] as const

/** Bentuk minimum yang kita pakai dari BarcodeDetector. */
interface Pendeteksi {
  detect(sumber: CanvasImageSource): Promise<{ rawValue: string }[]>
}

type StatusPemindai =
  | 'menyiapkan'
  | 'meminta-izin'
  | 'memindai'
  | 'ditolak'
  | 'tak-ada-kamera'
  | 'tak-didukung'
  | 'galat'

export interface HasilPemindai {
  video: React.RefObject<HTMLVideoElement>
  status: StatusPemindai
  /** Kalimat siap tampil untuk keadaan yang bukan "memindai". */
  pesan: string | null
  /** Senter, bila kameranya punya. */
  adaSenter: boolean
  senterMenyala: boolean
  ubahSenter: () => void
}

/** Apakah perangkat ini punya kamera sama sekali — dipakai menyembunyikan tombol. */
export function bisaMemindai(): boolean {
  return typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia
}

async function buatPendeteksi(): Promise<Pendeteksi> {
  const bawaan = (window as unknown as { BarcodeDetector?: new (o: unknown) => Pendeteksi })
    .BarcodeDetector
  if (bawaan) return new bawaan({ formats: [...FORMAT] })

  // Jalur cadangan: baru diunduh sekarang, bukan saat aplikasi dibuka.
  const { BarcodeDetector } = await import('barcode-detector/ponyfill')
  return new BarcodeDetector({ formats: [...FORMAT] }) as unknown as Pendeteksi
}

/**
 * @param aktif  pemindai hanya menyala saat dialognya terbuka — kamera yang
 *               menyala diam-diam menguras baterai dan bikin orang curiga.
 * @param onBaca dipanggil sekali per kode; pemanggil yang memutuskan berhenti.
 */
export function usePemindai(
  aktif: boolean,
  onBaca: (kode: string) => void,
): HasilPemindai {
  const video = useRef<HTMLVideoElement>(null)
  const [status, setStatus] = useState<StatusPemindai>('menyiapkan')
  const [adaSenter, setAdaSenter] = useState(false)
  const [senterMenyala, setSenterMenyala] = useState(false)

  const aliran = useRef<MediaStream | null>(null)
  const berhenti = useRef(false)

  // onBaca disimpan di ref, bukan jadi dependensi efek.
  //
  // Pemanggil hampir selalu memberi fungsi inline, yang berarti identitasnya
  // berubah tiap render. Kalau efeknya bergantung pada itu, kamera dimatikan
  // dan dinyalakan ulang setiap render — lampunya berkedip, pemindaian tidak
  // pernah sempat mengunci, dan baterai habis.
  const baca = useRef(onBaca)
  useEffect(() => {
    baca.current = onBaca
  }, [onBaca])
  // Kode yang sama tidak dilaporkan dua kali beruntun — kamera membaca puluhan
  // bingkai per detik dan akan menambahkan barang berkali-kali.
  const terakhir = useRef<{ kode: string; pada: number } | null>(null)

  const ubahSenter = useCallback(() => {
    const trek = aliran.current?.getVideoTracks()[0]
    if (!trek) return
    const nyala = !senterMenyala
    trek
      .applyConstraints({ advanced: [{ torch: nyala } as MediaTrackConstraintSet] })
      .then(() => setSenterMenyala(nyala))
      .catch(() => setAdaSenter(false))
  }, [senterMenyala])

  useEffect(() => {
    if (!aktif) return
    berhenti.current = false
    let bingkai = 0

    async function jalan() {
      if (!bisaMemindai()) {
        setStatus('tak-ada-kamera')
        return
      }

      let pendeteksi: Pendeteksi
      try {
        pendeteksi = await buatPendeteksi()
      } catch {
        setStatus('tak-didukung')
        return
      }

      setStatus('meminta-izin')
      try {
        aliran.current = await navigator.mediaDevices.getUserMedia({
          // Kamera belakang; di laptop pengaturan ini diabaikan dengan aman.
          video: { facingMode: { ideal: 'environment' }, width: { ideal: 1280 } },
        })
      } catch (e) {
        setStatus(
          e instanceof DOMException && e.name === 'NotAllowedError'
            ? 'ditolak'
            : 'tak-ada-kamera',
        )
        return
      }
      if (berhenti.current) return

      const trek = aliran.current.getVideoTracks()[0]
      setAdaSenter(!!trek && 'torch' in trek.getCapabilities())

      const el = video.current
      if (!el) return
      el.srcObject = aliran.current
      await el.play().catch(() => {})
      setStatus('memindai')

      const periksa = async () => {
        if (berhenti.current || !video.current) return
        // Tidak setiap bingkai: 30× per detik memboroskan baterai tanpa
        // membuat pembacaan lebih cepat.
        if (bingkai++ % 6 === 0 && video.current.readyState >= 2) {
          try {
            const kode = await pendeteksi.detect(video.current)
            const nilai = kode[0]?.rawValue?.trim()
            if (nilai) {
              const kini = Date.now()
              const sama =
                terakhir.current?.kode === nilai && kini - terakhir.current.pada < 2000
              if (!sama) {
                terakhir.current = { kode: nilai, pada: kini }
                baca.current(nilai)
              }
            }
          } catch {
            // Bingkai gagal dibaca itu biasa (buram, gelap) — coba lagi.
          }
        }
        if (!berhenti.current) requestAnimationFrame(() => void periksa())
      }
      void periksa()
    }

    void jalan().catch(() => setStatus('galat'))

    return () => {
      berhenti.current = true
      // Kamera WAJIB dimatikan saat dialog ditutup. Lampu kamera yang menyala
      // terus adalah cara tercepat membuat orang mencopot aplikasi.
      aliran.current?.getTracks().forEach((t) => t.stop())
      aliran.current = null
      setSenterMenyala(false)
      setStatus('menyiapkan')
    }
  }, [aktif])

  const pesan: Record<StatusPemindai, string | null> = {
    menyiapkan: 'Menyiapkan kamera…',
    'meminta-izin': 'Menunggu izin memakai kamera…',
    memindai: null,
    ditolak:
      'Aplikasi belum diizinkan memakai kamera. Buka pengaturan peramban untuk toko ini, izinkan Kamera, lalu coba lagi.',
    'tak-ada-kamera': 'Perangkat ini tidak punya kamera yang bisa dipakai.',
    'tak-didukung':
      'Peramban ini belum bisa membaca barcode. Ketik kodenya di kolom pencarian.',
    galat: 'Kamera tidak bisa dinyalakan. Ketik kodenya di kolom pencarian.',
  }

  return { video, status, pesan: pesan[status], adaSenter, senterMenyala, ubahSenter }
}
