import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Building2, Plus } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { crmApi, type Prospek } from '../api'
import { FITUR } from '@/lib/fitur'
import { BannerKunciFitur } from '@/bersama/komponen/kunci-fitur'

/**
 * Prospek (deal) per tahap.
 *
 * Papan bertahap, bukan tabel: yang ingin diketahui pemilik adalah "berapa
 * banyak yang hampir jadi", dan itu terbaca dari tinggi tumpukan tiap kolom.
 *
 * Catatan visibilitas: backend SUDAH menyaring — sales hanya melihat prospek
 * miliknya (crm.lead.view.own). Yang perlu UI lakukan hanyalah tidak
 * menampilkan penyaring "semua sales" kalau izinnya tidak ada.
 */
export function HalamanProspek() {
  const { boleh, punyaFitur } = useSesi()
  const [buatBaru, setBuatBaru] = useState(false)
  const [kalahUntuk, setKalahUntuk] = useState<Prospek | null>(null)

  const pipeline = useQuery({ queryKey: ['pipeline'], queryFn: crmApi.pipeline })
  const prospek = useQuery({ queryKey: ['prospek'], queryFn: () => crmApi.daftarProspek() })

  const bawaan = pipeline.data?.find((p) => p.is_default) ?? pipeline.data?.[0]
  const daftar = prospek.data?.data ?? []

  // Kelompokkan per tahap, mempertahankan urutan tahap dari server.
  const perTahap = useMemo(() => {
    const m = new Map<string, Prospek[]>()
    for (const t of bawaan?.stages ?? []) m.set(t.id, [])
    for (const d of daftar) {
      if (d.status !== 'open') continue
      const isi = m.get(d.stage_id)
      if (isi) isi.push(d)
    }
    return m
  }, [bawaan, daftar])

  const selesai = daftar.filter((d) => d.status !== 'open')
  const bolehUbah = boleh(IZIN.crmDealEdit)
  // Kunci paket menutup MEMULAI prospek baru; prospek yang sudah ada tetap
  // bisa dipindah tahap, dimenangkan, atau ditutup (server sama).
  const bolehBaru = bolehUbah && punyaFitur(FITUR.crmFreelance)

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Prospek</h1>
        {bolehBaru && (
          <Tombol onClick={() => setBuatBaru(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Prospek
          </Tombol>
        )}
      </header>

      <BannerKunciFitur
        fitur={FITUR.crmFreelance}
        nama="CRM"
        penjelasan="Prospek yang sudah ada tetap bisa dilihat, dipindah tahap, dan ditutup — hanya menambah prospek baru yang terkunci."
      />

      {pipeline.isLoading || prospek.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : !bawaan ? (
        <KeadaanKosong
          ikon={Building2}
          judul="Belum ada alur prospek"
          penjelasan="Alur tahapan (misalnya: Baru → Penawaran → Menang) belum disiapkan untuk usaha ini."
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Building2}
          judul="Belum ada prospek"
          penjelasan="Catat calon pelanggan yang sedang Anda dekati supaya tidak ada yang terlupa ditindaklanjuti."
          aksi={bolehBaru ? { label: 'Tambah Prospek', onKlik: () => setBuatBaru(true) } : undefined}
        />
      ) : (
        <div className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-3 overflow-x-auto px-4 pb-2 sm:mx-0 sm:snap-none sm:px-0">
          {/* HP: satu tahap selebar ±85% layar, berhenti tepat di kolom (snap) —
              dulu kolom berhenti di tengah dan tombolnya terpotong ("→ Pe").
              Layar lebar: kolom 16rem berjejer seperti papan kanban biasa. */}
          {bawaan.stages.map((t) => {
            const isi = perTahap.get(t.id) ?? []
            const nilai = isi.reduce((j, d) => j + d.value, 0)
            return (
              <section key={t.id} className="flex w-[85%] shrink-0 snap-start flex-col gap-2 sm:w-64">
                <div className="flex items-baseline justify-between gap-2">
                  <h2 className="text-label font-semibold text-teks-utama">{t.name}</h2>
                  <span className="text-keterangan tabular-nums text-teks-redup">
                    {isi.length}
                  </span>
                </div>
                <p className="text-keterangan tabular-nums text-teks-redup">
                  {formatRupiah(nilai)}
                </p>

                <ul className="flex flex-col gap-2">
                  {isi.map((d) => (
                    <li key={d.id}>
                      <KartuProspek
                        prospek={d}
                        tahap={bawaan.stages}
                        bolehUbah={bolehUbah}
                        onKalah={() => setKalahUntuk(d)}
                      />
                    </li>
                  ))}
                  {isi.length === 0 && (
                    <li className="rounded-kartu border border-dashed border-garis p-3 text-center text-keterangan text-teks-redup">
                      Kosong
                    </li>
                  )}
                </ul>
              </section>
            )
          })}
        </div>
      )}

      {selesai.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">Sudah selesai</h2>
          <Kartu className="divide-y divide-garis">
            {selesai.map((d) => (
              <div key={d.id} className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-medium text-teks-utama">{d.title}</p>
                  <p className="text-keterangan text-teks-redup">
                    {d.closed_at && formatTanggal(d.closed_at)}
                    {d.lost_reason && ` · ${d.lost_reason}`}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(d.value)}
                  </p>
                  <p
                    className={cn(
                      'text-keterangan font-medium',
                      d.status === 'won' ? 'text-hijau-700' : 'text-bahaya-teks',
                    )}
                  >
                    {d.status === 'won' ? 'Menang' : 'Kalah'}
                  </p>
                </div>
              </div>
            ))}
          </Kartu>
        </section>
      )}

      {buatBaru && <DialogProspek onTutup={() => setBuatBaru(false)} />}
      {kalahUntuk && (
        <DialogKalah prospek={kalahUntuk} onTutup={() => setKalahUntuk(null)} />
      )}
    </div>
  )
}

function KartuProspek({
  prospek,
  tahap,
  bolehUbah,
  onKalah,
}: {
  prospek: Prospek
  tahap: { id: string; name: string; is_won: boolean }[]
  bolehUbah: boolean
  onKalah: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()

  const pindah = useMutation({
    mutationFn: (stage_id: string) => crmApi.ubahProspek(prospek.id, { stage_id }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['prospek'] })
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Gagal memindahkan.'),
  })

  const menang = useMutation({
    mutationFn: () => crmApi.menang(prospek.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['prospek'] })
      toast.berhasil(`${prospek.title} ditandai menang. Selamat!`)
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  const indeks = tahap.findIndex((t) => t.id === prospek.stage_id)
  const berikut = tahap[indeks + 1]

  return (
    <Kartu className="flex flex-col gap-2 p-3">
      <p className="font-medium text-teks-utama">{prospek.title}</p>
      <p className="text-keterangan tabular-nums text-teks-redup">
        {formatRupiah(prospek.value)}
        {prospek.expected_close_date &&
          ` · target ${formatTanggal(prospek.expected_close_date)}`}
      </p>

      {bolehUbah && (
        <div className="flex flex-wrap gap-1.5">
          {berikut &&
            (berikut.is_won ? (
              <Tombol ukuran="padat" memuat={menang.isPending} onClick={() => menang.mutate()}>
                Menang
              </Tombol>
            ) : (
              <Tombol
                jenis="kedua"
                ukuran="padat"
                memuat={pindah.isPending}
                onClick={() => pindah.mutate(berikut.id)}
              >
                → {berikut.name}
              </Tombol>
            ))}
          {/* Tombol "kalah" sengaja tersier dan berjarak dari tombol maju. */}
          <Tombol jenis="teks" ukuran="padat" onClick={onKalah}>
            Kalah
          </Tombol>
        </div>
      )}
    </Kartu>
  )
}

function DialogProspek({ onTutup }: { onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [judul, setJudul] = useState('')
  const [nilai, setNilai] = useState(0)
  const [target, setTarget] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const buat = useMutation({
    mutationFn: () =>
      crmApi.buatProspek({
        title: judul.trim(),
        value: nilai,
        expected_close_date: target || undefined,
      }),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['prospek'] })
      toast.berhasil(`Prospek ${d.title} dicatat.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !buat.isPending && onTutup()}>
      <IsiDialog judul="Tambah Prospek">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            buat.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama prospek"
            placeholder="Contoh: Katering kantor Bu Rina"
            value={judul}
            onChange={(e) => setJudul(e.target.value)}
            autoFocus
            required
          />
          <KolomUang
            label="Perkiraan nilai"
            nilai={nilai}
            onNilai={setNilai}
            bantuan="Berapa kira-kira nilainya bila jadi. Boleh Rp 0 kalau belum tahu."
          />
          <Kolom
            label="Target selesai"
            type="date"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            bantuan="Boleh dikosongkan."
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={buat.isPending} disabled={!judul.trim()}>
              Simpan Prospek
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={buat.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}

function DialogKalah({ prospek, onTutup }: { prospek: Prospek; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const kalah = useMutation({
    mutationFn: () => crmApi.kalah(prospek.id, alasan.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['prospek'] })
      toast.tampilkan(`${prospek.title} ditandai kalah.`, 'info')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !kalah.isPending && onTutup()}>
      <IsiDialog
        judul={`Tandai "${prospek.title}" kalah?`}
        keterangan="Alasannya dicatat supaya nanti terlihat pola kenapa prospek gagal — itu yang bisa diperbaiki."
      >
        <Kolom
          label="Kenapa kalah?"
          placeholder="Contoh: harga terlalu mahal"
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
          autoFocus
          required
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={kalah.isPending}
            disabled={!alasan.trim()}
            onClick={() => {
              setGalat(null)
              kalah.mutate()
            }}
          >
            Simpan
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={kalah.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
