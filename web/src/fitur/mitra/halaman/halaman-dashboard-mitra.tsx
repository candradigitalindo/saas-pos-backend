import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Info, Plus, ShieldCheck, Store } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { mitraApi } from '../api'
import { useSesiMitra } from '../sesi-mitra'

/**
 * Beranda mitra — satu pertanyaan, satu jawaban (ui/05-ALUR-UTAMA.md §9).
 *
 * Yang dicari mitra cuma satu: komisi saya kapan cair dan berapa. Karena itu
 * angka itu yang paling besar dan paling atas; sisanya menyusul.
 */
export function HalamanDashboardMitra() {
  const { mitra } = useSesiMitra()
  const [buatProspek, setBuatProspek] = useState(false)

  const dashboard = useQuery({
    queryKey: ['mitra-dashboard'],
    queryFn: mitraApi.dashboard,
  })

  const merchant = useQuery({
    queryKey: ['mitra-merchant'],
    queryFn: () => mitraApi.merchantBinaan(),
  })

  const d = dashboard.data
  const prospekTotal = Object.values(d?.leads_by_status ?? {}).reduce((a, b) => a + b, 0)

  return (
    <div className="flex flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">
          Halo, {mitra?.name ?? 'Mitra'}
        </h1>
        <p className="text-label text-teks-sekunder">
          Kode referral Anda:{' '}
          <strong className="tabular-nums text-teks-utama">{mitra?.referral_code}</strong>
        </p>
      </header>

      {dashboard.isLoading ? (
        <KerangkaKartuAngka />
      ) : (
        <Kartu className="p-4">
          <p className="text-label font-medium text-teks-sekunder">
            Komisi siap cair
          </p>
          <p className="mt-1 text-angka font-extrabold tabular-nums text-teks-utama">
            {formatRupiah(d?.commission_approved ?? 0)}
          </p>
          {/* Inilah yang dicari — jadi disebut lebih dulu daripada rincian. */}
          <p className="mt-1 text-keterangan text-teks-redup">
            {d?.last_payout
              ? `Pencairan terakhir ${formatRupiah(d.last_payout.net_amount)} pada ${formatTanggal(d.last_payout.paid_at ?? d.last_payout.period_end)}`
              : 'Belum pernah ada pencairan.'}
          </p>
          {(d?.commission_held ?? 0) > 0 && (
            <p className="mt-2 rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan text-jingga-700">
              {formatRupiah(d!.commission_held)} masih menunggu diperiksa dan belum
              bisa dicairkan.
            </p>
          )}
        </Kartu>
      )}

      <div className="grid grid-cols-3 gap-3">
        <KartuKecil label="Prospek" nilai={prospekTotal} />
        <KartuKecil label="Merchant aktif" nilai={d?.merchants_active ?? 0} />
        <KartuKecil label="Total merchant" nilai={d?.merchants_total ?? 0} />
      </div>

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">
          Merchant binaan
        </h2>

        {merchant.isLoading ? (
          <KerangkaBaris jumlah={3} />
        ) : (merchant.data?.length ?? 0) === 0 ? (
          <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
            <Store className="h-10 w-10 text-teks-redup" aria-hidden strokeWidth={1.5} />
            <p className="text-isi text-teks-sekunder">
              Belum ada merchant yang berlangganan lewat kode Anda.
            </p>
          </Kartu>
        ) : (
          <Kartu className="divide-y divide-garis">
            {merchant.data?.map((m) => (
              <div
                key={m.tenant_id}
                className="flex items-center justify-between gap-3 p-4"
              >
                <div className="min-w-0">
                  <p className="truncate font-medium text-teks-utama">
                    {m.business_name}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {m.current_period_end
                      ? `Jatuh tempo ${formatTanggal(m.current_period_end)}`
                      : 'Belum berlangganan'}
                  </p>
                </div>
                {m.is_active ? (
                  <LencanaStatus nada="berhasil" anak="Aktif" />
                ) : (
                  <LencanaStatus nada="netral" anak="Tidak aktif" />
                )}
              </div>
            ))}
          </Kartu>
        )}

        {/* Batas privasi ditulis PERMANEN, bukan disembunyikan. Ia melindungi
            merchant sekaligus menjawab lebih dulu pertanyaan mitra "kok datanya
            cuma segini?" (ui/04-PETA-LAYAR.md). */}
        <p className="flex items-start gap-2 rounded-kontrol bg-info-teks/10 px-3 py-2 text-keterangan text-info-teks">
          <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          Anda hanya dapat melihat status langganan merchant. Data penjualan,
          produk, dan pelanggan mereka bersifat rahasia.
        </p>
      </section>

      <Tombol onClick={() => setBuatProspek(true)} className="self-start">
        <Plus className="h-5 w-5" aria-hidden />
        Daftarkan Prospek
      </Tombol>

      {buatProspek && <DialogProspek onTutup={() => setBuatProspek(false)} />}
    </div>
  )
}

function KartuKecil({ label, nilai }: { label: string; nilai: number }) {
  return (
    <Kartu className="p-3 text-center">
      <p className="text-keterangan text-teks-sekunder">{label}</p>
      <p className="mt-1 text-judul font-bold tabular-nums text-teks-utama">{nilai}</p>
    </Kartu>
  )
}

function DialogProspek({ onTutup }: { onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState('')
  const [kontak, setKontak] = useState('')
  const [hp, setHp] = useState('')
  const [kota, setKota] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const daftar = useMutation({
    mutationFn: () =>
      mitraApi.daftarkanProspek({
        business_name: nama.trim(),
        contact_name: kontak.trim() || undefined,
        phone: hp.trim(),
        city: kota.trim() || undefined,
      }),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['mitra-dashboard'] })
      qc.invalidateQueries({ queryKey: ['mitra-prospek'] })
      toast.berhasil(
        `${p.business_name} terdaftar. Atribusinya berlaku sampai ${formatTanggal(p.attribution_expires_at)}.`,
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !daftar.isPending && onTutup()}>
      <IsiDialog
        judul="Daftarkan Prospek"
        keterangan="Catat calon merchant sekarang supaya komisinya tetap jadi milik Anda saat mereka berlangganan."
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            daftar.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama usaha"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            autoFocus
            required
          />
          <Kolom
            label="Nama pemilik"
            value={kontak}
            onChange={(e) => setKontak(e.target.value)}
            bantuan="Boleh dikosongkan."
          />
          <Kolom
            label="Nomor HP"
            type="tel"
            inputMode="tel"
            value={hp}
            onChange={(e) => setHp(e.target.value)}
            required
          />
          <Kolom
            label="Kota"
            value={kota}
            onChange={(e) => setKota(e.target.value)}
            bantuan="Boleh dikosongkan."
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol
              type="submit"
              memuat={daftar.isPending}
              disabled={!nama.trim() || !hp.trim()}
            >
              <ShieldCheck className="h-5 w-5" aria-hidden />
              Daftarkan
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={daftar.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
