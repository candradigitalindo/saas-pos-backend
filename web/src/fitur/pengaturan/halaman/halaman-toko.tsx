import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Store } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { hitungTotal } from '@/bersama/util/total'
import { pecahanKePersen, persenKePecahan } from '@/bersama/util/persen'
import { formatRupiah } from '@/bersama/util/uang'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import type { Toko } from '@/bersama/tipe/organisasi'
import { pengaturanApi } from '../api'
import { BannerKunciFitur } from '@/bersama/komponen/kunci-fitur'
import { FITUR } from '@/lib/fitur'

const ZONA = [
  { nilai: 'Asia/Jakarta', label: 'WIB — Jakarta, Sumatera, Jawa' },
  { nilai: 'Asia/Makassar', label: 'WITA — Bali, Kalimantan, Sulawesi' },
  { nilai: 'Asia/Jayapura', label: 'WIT — Maluku, Papua' },
]

/**
 * Toko / cabang.
 *
 * Zona waktu dan jam tutup buku ada di sini karena keduanya menentukan
 * `business_date` — hari penjualan mana sebuah transaksi masuk. Warung yang
 * tutup jam 2 pagi tidak ingin penjualan tengah malamnya jatuh ke hari berikutnya.
 *
 * Pajak & biaya layanan juga PER CABANG: satu usaha bisa punya kedai kopi yang
 * memungut PB1 dan toko kelontong yang tidak.
 */
export function HalamanToko() {
  const { tokoAktif, gantiToko, profil, punyaFitur } = useSesi()
  const [formUntuk, setFormUntuk] = useState<Toko | 'baru' | null>(null)

  const toko = useQuery({
    queryKey: ['outlets'],
    queryFn: () => pengaturanApi.daftarToko(),
  })

  const daftar = toko.data?.data ?? []
  // Batas cabang paket (sudah memperhitungkan fitur multi_outlet; null =
  // tanpa batas). Profil lama tanpa `plan` → tidak dibatasi di layar; server
  // tetap menegakkan.
  const batasCabang = profil?.plan ? profil.plan.max_outlets : null
  const penuh = !!toko.data && batasCabang !== null && daftar.length >= batasCabang

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Toko & Cabang</h1>
        {!penuh && (
          <Tombol onClick={() => setFormUntuk('baru')}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Cabang
          </Tombol>
        )}
      </header>

      {/* Tanpa fitur banyak cabang: ajakan naik paket. Dengan fiturnya tapi
          kuota penuh: sebut batasnya. Cabang yang sudah ada tetap bisa diubah. */}
      {penuh &&
        (punyaFitur(FITUR.banyakCabang) ? (
          <p className="rounded-kontrol border border-garis bg-permukaan-2 px-3 py-2 text-label text-teks-sekunder">
            Paket {profil?.plan?.name} dibatasi {batasCabang} cabang. Naikkan paket di menu
            Langganan untuk menambah cabang lagi.
          </p>
        ) : (
          <BannerKunciFitur
            fitur={FITUR.banyakCabang}
            nama="Menambah cabang"
            penjelasan="Cabang yang sudah ada tetap bisa dipakai dan diubah."
          />
        ))}

      {toko.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((t) => (
            <li key={t.id}>
              {/* Boleh melipat: di HP kolom teks (alamat, tutup buku, tarif)
                  tidak terjepit jadi 5 baris di samping lencana & tombol —
                  keduanya turun ke baris sendiri bila tak muat. */}
              <Kartu className="flex flex-wrap items-center justify-between gap-3 p-4">
                <div className="min-w-0 flex-1 basis-52">
                  <p className="flex items-center gap-2 font-semibold text-teks-utama">
                    <Store className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
                    {t.name}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {t.address || 'Tanpa alamat'} · tutup buku {t.business_day_start}
                    {ringkasTarif(t) && ` · ${ringkasTarif(t)}`}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {t.id === tokoAktif ? (
                    <LencanaStatus nada="berhasil" anak="Sedang dipakai" />
                  ) : (
                    <Tombol jenis="teks" ukuran="padat" onClick={() => gantiToko(t.id)}>
                      Pakai toko ini
                    </Tombol>
                  )}
                  <Tombol jenis="kedua" ukuran="padat" onClick={() => setFormUntuk(t)}>
                    Ubah
                  </Tombol>
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {formUntuk && (
        <DialogToko
          awal={formUntuk === 'baru' ? null : formUntuk}
          onTutup={() => setFormUntuk(null)}
        />
      )}
    </div>
  )
}

function DialogToko({ awal, onTutup }: { awal: Toko | null; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState(awal?.name ?? '')
  const [alamat, setAlamat] = useState(awal?.address ?? '')
  const [hp, setHp] = useState(awal?.phone ?? '')
  const [zona, setZona] = useState(awal?.timezone ?? 'Asia/Jakarta')
  const [jamTutup, setJamTutup] = useState(awal?.business_day_start ?? '00:00')
  const [pajakAktif, setPajakAktif] = useState(awal?.tax_enabled ?? false)
  const [tarifPajak, setTarifPajak] = useState(() => pecahanKePersen(awal?.tax_rate))
  // Bawaan "sudah termasuk": itu yang dipahami pembeli dari label harga di rak,
  // dan sama dengan bawaan server.
  const [caraPajak, setCaraPajak] = useState<'inklusif' | 'eksklusif'>(
    awal && !awal.tax_inclusive ? 'eksklusif' : 'inklusif',
  )
  const [tarifLayanan, setTarifLayanan] = useState(() =>
    pecahanKePersen(awal?.service_charge_rate),
  )
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  // Persen yang diketik → pecahan untuk server. null = isiannya belum sah.
  const pecahanPajak = pajakAktif ? persenKePecahan(tarifPajak) : '0'
  const pecahanLayanan = persenKePecahan(tarifLayanan)
  const pajakBelumDiisi = pajakAktif && pecahanPajak === '0'
  const galatPajak =
    pecahanPajak === null
      ? 'Isi persen 0–99, paling banyak dua angka di belakang koma. Contoh: 10 atau 7,5.'
      : pajakBelumDiisi
        ? 'Isi tarifnya, atau matikan pajak.'
        : undefined
  const galatLayanan =
    pecahanLayanan === null
      ? 'Isi persen 0–99, paling banyak dua angka di belakang koma. Contoh: 5.'
      : undefined

  // Contoh hitungan dengan KALKULATOR YANG SAMA dengan kasir — pemilik melihat
  // akibatnya dalam rupiah sebelum menyimpan, bukan hanya angka persen.
  const contoh = useMemo(() => {
    if (pecahanPajak === null || pecahanLayanan === null) return null
    const r = hitungTotal([{ harga: 10_000, qty: '1', diskon: 0 }], {
      tax_enabled: pajakAktif,
      tax_rate: pecahanPajak,
      tax_inclusive: caraPajak === 'inklusif',
      service_charge_rate: pecahanLayanan,
    })
    return r
  }, [pajakAktif, pecahanPajak, caraPajak, pecahanLayanan])

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        address: alamat.trim() || undefined,
        phone: hp.trim() || undefined,
        timezone: zona,
        business_day_start: jamTutup,
        tax_enabled: pajakAktif,
        // Tarif tetap dikirim walau pajak dimatikan, supaya menyalakannya lagi
        // nanti tidak meminta pemilik mengingat angkanya.
        tax_rate: (pajakAktif ? pecahanPajak : persenKePecahan(tarifPajak)) ?? '0',
        tax_inclusive: caraPajak === 'inklusif',
        service_charge_rate: pecahanLayanan ?? '0',
      }
      return awal ? pengaturanApi.ubahToko(awal.id, isi) : pengaturanApi.buatToko(isi)
    },
    onSuccess: (t) => {
      qc.invalidateQueries({ queryKey: ['outlets'] })
      qc.invalidateQueries({ queryKey: ['me'] })
      toast.berhasil(awal ? `${t.name} diperbarui.` : `Cabang ${t.name} ditambahkan.`)
      onTutup()
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else setGalat('Terjadi kesalahan. Coba lagi.')
    },
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog judul={awal ? 'Ubah Toko' : 'Tambah Cabang'}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            setKolomGalat({})
            simpan.mutate()
          }}
          className="flex max-h-[65vh] flex-col gap-4 overflow-y-auto"
          noValidate
        >
          <Kolom
            label="Nama toko"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            galat={galatKolom(kolomGalat, 'name')}
            autoFocus
            required
          />
          <Kolom
            label="Alamat"
            value={alamat}
            onChange={(e) => setAlamat(e.target.value)}
            bantuan="Boleh dikosongkan. Muncul di struk bila diisi."
            galat={galatKolom(kolomGalat, 'address')}
          />
          <Kolom
            label="Nomor HP toko"
            type="tel"
            inputMode="tel"
            value={hp}
            onChange={(e) => setHp(e.target.value)}
            bantuan="Boleh dikosongkan."
            galat={galatKolom(kolomGalat, 'phone')}
          />
          <Pilihan
            label="Zona waktu"
            value={zona}
            onChange={(e) => setZona(e.target.value)}
            galat={galatKolom(kolomGalat, 'timezone')}
          >
            {ZONA.map((z) => (
              <option key={z.nilai} value={z.nilai}>
                {z.label}
              </option>
            ))}
          </Pilihan>
          <Kolom
            label="Jam tutup buku"
            type="time"
            value={jamTutup}
            onChange={(e) => setJamTutup(e.target.value)}
            bantuan="Penjualan sebelum jam ini masih dihitung sebagai hari sebelumnya. Isi 00:00 bila toko tutup sebelum tengah malam."
            galat={galatKolom(kolomGalat, 'business_day_start')}
          />

          {/* min-w-0 WAJIB: <fieldset> bawaan peramban ber-min-inline-size
              min-content, jadi ia melebar mengikuti isi yang tak bisa dilipat
              (kontrol segmen) dan seluruh form ikut tergulir ke samping di HP. */}
          <fieldset className="flex min-w-0 flex-col gap-3 rounded-kartu border border-garis p-3">
            <legend className="px-1 text-label font-medium text-teks-sekunder">
              Pajak & biaya layanan
            </legend>

            <label className="flex min-h-12 cursor-pointer items-center gap-3">
              <input
                type="checkbox"
                checked={pajakAktif}
                onChange={(e) => setPajakAktif(e.target.checked)}
                className="h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
              />
              <span className="text-label text-teks-utama">Pungut pajak (PB1 / PPN)</span>
            </label>

            {pajakAktif && (
              <>
                <Kolom
                  label="Tarif pajak"
                  inputMode="decimal"
                  akhiran="%"
                  value={tarifPajak}
                  onChange={(e) => setTarifPajak(e.target.value)}
                  bantuan="Contoh: 10 untuk PB1 restoran, 11 atau 12 untuk PPN."
                  galat={galatPajak ?? galatKolom(kolomGalat, 'tax_rate')}
                />
                <div className="flex flex-col gap-1">
                  <span className="text-label font-medium text-teks-sekunder">
                    Harga di etalase sudah termasuk pajak?
                  </span>
                  <SegmenPilihan
                    label="Harga di etalase sudah termasuk pajak?"
                    pilihan={[
                      ['inklusif', 'Ya, sudah'],
                      ['eksklusif', 'Belum, ditambahkan'],
                    ]}
                    nilai={caraPajak}
                    onPilih={setCaraPajak}
                  />
                </div>
              </>
            )}

            <Kolom
              label="Biaya layanan"
              inputMode="decimal"
              akhiran="%"
              value={tarifLayanan}
              onChange={(e) => setTarifLayanan(e.target.value)}
              bantuan="Kosongkan bila tidak ada. Umumnya 5–10% di restoran."
              galat={galatLayanan ?? galatKolom(kolomGalat, 'service_charge_rate')}
            />

            {contoh && (
              <p className="rounded-kontrol bg-permukaan-2 px-3 py-2 text-label text-teks-sekunder">
                {kalimatContoh(contoh, pajakAktif && caraPajak === 'inklusif')}
              </p>
            )}
            <p className="text-keterangan text-teks-redup">
              Berlaku untuk transaksi berikutnya. Kasir yang sedang terbuka memakai
              tarif baru saat membuka layar bayar berikutnya.
            </p>
          </fieldset>

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol
              type="submit"
              memuat={simpan.isPending}
              disabled={!nama.trim() || !!galatPajak || !!galatLayanan}
            >
              Simpan
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}

/** "PB1 10% · layanan 5%" untuk baris daftar toko; kosong bila tidak ada. */
function ringkasTarif(t: Toko): string {
  const bagian: string[] = []
  const pajak = pecahanKePersen(t.tax_rate)
  if (t.tax_enabled && pajak) bagian.push(`pajak ${pajak}%${t.tax_inclusive ? ' (termasuk)' : ''}`)
  const layanan = pecahanKePersen(t.service_charge_rate)
  if (layanan) bagian.push(`layanan ${layanan}%`)
  return bagian.join(' · ')
}

/** Kalimat contoh hitungan untuk barang Rp 10.000. */
function kalimatContoh(
  r: { total: number; tax_amount: number; service_amount: number },
  pajakTermasuk: boolean,
): string {
  if (r.tax_amount === 0 && r.service_amount === 0) {
    return 'Contoh: barang seharga Rp 10.000 dibayar pembeli Rp 10.000 — tanpa tambahan.'
  }
  const rincian: string[] = []
  if (r.tax_amount > 0) {
    rincian.push(
      pajakTermasuk
        ? `sudah termasuk pajak ${formatRupiah(r.tax_amount)}`
        : `pajak ${formatRupiah(r.tax_amount)}`,
    )
  }
  if (r.service_amount > 0) rincian.push(`biaya layanan ${formatRupiah(r.service_amount)}`)
  return `Contoh: barang seharga Rp 10.000 dibayar pembeli ${formatRupiah(r.total)} (${rincian.join(', ')}).`
}
