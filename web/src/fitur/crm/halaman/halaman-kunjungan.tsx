import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { MapPin, Navigation } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { antrekan } from '@/lib/offline/antrean'
import { useSinkron } from '@/lib/offline/mesin'
import { formatJam, formatTanggal } from '@/bersama/util/tanggal'
import { usePelanggan } from '@/bersama/hooks/use-katalog'
import { crmApi, type Kunjungan } from '../api'

const HASIL = [
  { nilai: 'order', label: 'Pesan barang' },
  { nilai: 'no_order', label: 'Tidak pesan' },
  { nilai: 'closed', label: 'Toko tutup' },
  { nilai: 'rejected', label: 'Ditolak' },
]

/**
 * Kunjungan sales lapangan.
 *
 * Dipakai DI JALAN, dengan sinyal putus-putus. Karena itu check-in dan
 * check-out mengikuti pola yang sama dengan kasir: id dibuat klien, dan bila
 * jaringan mati operasinya masuk antrean lalu dikirim lewat /sync/push —
 * bukan gagal di depan mata sales yang sedang berdiri di depan toko.
 */
export function HalamanKunjungan() {
  const sinkron = useSinkron()
  const [checkinUntuk, setCheckinUntuk] = useState(false)
  const [checkoutUntuk, setCheckoutUntuk] = useState<Kunjungan | null>(null)

  const kunjungan = useQuery({
    queryKey: ['kunjungan'],
    queryFn: () => crmApi.daftarKunjungan(),
  })

  const pelanggan = usePelanggan()

  const namaPelanggan = useMemo(() => {
    const m = new Map<string, string>()
    for (const p of pelanggan.data?.data ?? []) m.set(p.id, p.name)
    return m
  }, [pelanggan.data])

  const daftar = kunjungan.data?.data ?? []
  const berjalan = daftar.filter((k) => k.checkin_at && !k.checkout_at)

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Kunjungan</h1>
        <Tombol onClick={() => setCheckinUntuk(true)}>
          <MapPin className="h-5 w-5" aria-hidden />
          Check-in
        </Tombol>
      </header>

      <StatusKoneksi menunggu={sinkron.menunggu} />

      {berjalan.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Sedang berlangsung
          </h2>
          {berjalan.map((k) => (
            <Kartu key={k.id} className="flex items-center justify-between gap-3 border-utama bg-sorot p-4">
              <div className="min-w-0">
                <p className="truncate font-semibold text-teks-utama">
                  {namaPelanggan.get(k.customer_id) ?? 'Pelanggan'}
                </p>
                <p className="text-keterangan text-teks-redup">
                  Masuk {k.checkin_at ? formatJam(k.checkin_at) : '—'}
                </p>
              </div>
              <Tombol ukuran="padat" onClick={() => setCheckoutUntuk(k)}>
                Selesai
              </Tombol>
            </Kartu>
          ))}
        </section>
      )}

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">Riwayat</h2>

        {kunjungan.isLoading ? (
          <KerangkaBaris jumlah={4} />
        ) : daftar.length === 0 ? (
          <KeadaanKosong
            ikon={Navigation}
            judul="Belum ada kunjungan"
            penjelasan="Tekan Check-in saat Anda tiba di toko pelanggan. Bisa dilakukan walau sedang tidak ada sinyal."
            aksi={{ label: 'Check-in Sekarang', onKlik: () => setCheckinUntuk(true) }}
          />
        ) : (
          <Kartu className="divide-y divide-garis">
            {daftar
              .filter((k) => k.checkout_at)
              .map((k) => (
                <div key={k.id} className="flex items-center justify-between gap-3 p-4">
                  <div className="min-w-0">
                    <p className="truncate font-medium text-teks-utama">
                      {namaPelanggan.get(k.customer_id) ?? 'Pelanggan'}
                    </p>
                    <p className="text-keterangan text-teks-redup">
                      {formatTanggal(k.business_date)}
                      {k.checkin_at && ` · ${formatJam(k.checkin_at)}`}
                      {k.no_order_reason && ` · ${k.no_order_reason}`}
                    </p>
                  </div>
                  <HasilKunjungan hasil={k.result} />
                </div>
              ))}
          </Kartu>
        )}
      </section>

      {checkinUntuk && (
        <DialogCheckin
          pelanggan={pelanggan.data?.data ?? []}
          onTutup={() => setCheckinUntuk(false)}
        />
      )}
      {checkoutUntuk && (
        <DialogCheckout kunjungan={checkoutUntuk} onTutup={() => setCheckoutUntuk(null)} />
      )}
    </div>
  )
}

function DialogCheckin({
  pelanggan,
  onTutup,
}: {
  pelanggan: { id: string; name: string }[]
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const sinkron = useSinkron()
  const [pelangganId, setPelangganId] = useState(pelanggan[0]?.id ?? '')
  const [galat, setGalat] = useState<string | null>(null)

  const checkin = useMutation({
    mutationFn: async () => {
      // ULID dibuat KLIEN — kunci yang sama dipakai baik lewat POST langsung
      // maupun lewat antrean, jadi kunjungan tidak pernah tercatat dua kali.
      const id = ulid()
      const sekarang = new Date().toISOString()
      const posisi = await ambilPosisi()
      const isi = {
        id,
        customer_id: pelangganId,
        checkin_at: sekarang,
        ...(posisi ?? {}),
      }

      try {
        await crmApi.checkin(isi)
        return { diantre: false }
      } catch (e) {
        if (!(e instanceof GalatAPI) || !e.bisaDiantre) throw e
        await antrekan({
          id,
          op: 'visit.upsert',
          payload: { ...isi, result: 'pending' },
          ringkasan: `Kunjungan ke ${pelanggan.find((p) => p.id === pelangganId)?.name ?? 'pelanggan'}`,
          nominal: 0,
        })
        return { diantre: true }
      }
    },
    onSuccess: ({ diantre }) => {
      qc.invalidateQueries({ queryKey: ['kunjungan'] })
      sinkron.segarkan()
      toast.berhasil(
        diantre
          ? 'Check-in tersimpan di HP dan akan dikirim otomatis saat ada sinyal.'
          : 'Check-in tercatat.',
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !checkin.isPending && onTutup()}>
      <IsiDialog
        judul="Check-in Kunjungan"
        keterangan="Tekan saat Anda sudah sampai di toko pelanggan."
      >
        <Pilihan
          label="Pelanggan"
          value={pelangganId}
          onChange={(e) => setPelangganId(e.target.value)}
          required
        >
          {pelanggan.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Pilihan>

        <p className="text-keterangan text-teks-redup">
          Lokasi Anda ikut dicatat bila diizinkan peramban. Tanpa lokasi pun
          check-in tetap bisa dilakukan.
        </p>

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={checkin.isPending}
            labelMemuat="Mencatat…"
            disabled={!pelangganId}
            onClick={() => {
              setGalat(null)
              checkin.mutate()
            }}
          >
            <MapPin className="h-5 w-5" aria-hidden />
            Check-in
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={checkin.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

function DialogCheckout({
  kunjungan,
  onTutup,
}: {
  kunjungan: Kunjungan
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [hasil, setHasil] = useState('order')
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const selesai = useMutation({
    mutationFn: () =>
      crmApi.checkout(kunjungan.id, {
        result: hasil,
        no_order_reason: hasil === 'no_order' ? alasan.trim() || undefined : undefined,
        checkout_at: new Date().toISOString(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['kunjungan'] })
      toast.berhasil('Kunjungan selesai dicatat.')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !selesai.isPending && onTutup()}>
      <IsiDialog judul="Selesaikan Kunjungan">
        <Pilihan label="Hasil kunjungan" value={hasil} onChange={(e) => setHasil(e.target.value)}>
          {HASIL.map((h) => (
            <option key={h.nilai} value={h.nilai}>
              {h.label}
            </option>
          ))}
        </Pilihan>

        {hasil === 'no_order' && (
          <Kolom
            label="Kenapa tidak pesan?"
            placeholder="Contoh: stoknya masih banyak"
            value={alasan}
            onChange={(e) => setAlasan(e.target.value)}
            bantuan="Dicatat supaya terlihat pola alasan yang paling sering muncul."
          />
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={selesai.isPending}
            onClick={() => {
              setGalat(null)
              selesai.mutate()
            }}
          >
            Simpan
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={selesai.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

function HasilKunjungan({ hasil }: { hasil: string }) {
  switch (hasil) {
    case 'order':
      return <LencanaStatus nada="berhasil" anak="Pesan" />
    case 'no_order':
      return <LencanaStatus nada="menunggu" anak="Tidak pesan" />
    case 'closed':
      return <LencanaStatus nada="netral" anak="Tutup" />
    case 'rejected':
      return <LencanaStatus nada="bahaya" anak="Ditolak" />
    default:
      return <LencanaStatus nada="netral" anak="Berlangsung" />
  }
}

/**
 * Lokasi saat check-in. Ditunggu paling lama 5 detik dan kegagalannya
 * DIABAIKAN: sales yang berdiri di depan toko tidak boleh terhalang oleh GPS
 * yang lambat atau izin lokasi yang ditolak.
 */
function ambilPosisi(): Promise<{ checkin_lat: string; checkin_lng: string } | null> {
  if (!('geolocation' in navigator)) return Promise.resolve(null)
  return new Promise((selesai) => {
    navigator.geolocation.getCurrentPosition(
      (p) =>
        selesai({
          checkin_lat: String(p.coords.latitude),
          checkin_lng: String(p.coords.longitude),
        }),
      () => selesai(null),
      { timeout: 5000, maximumAge: 60_000 },
    )
  })
}
