import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Store } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import type { Toko } from '@/bersama/tipe/organisasi'
import { pengaturanApi } from '../api'

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
 */
export function HalamanToko() {
  const { tokoAktif, gantiToko } = useSesi()
  const [formUntuk, setFormUntuk] = useState<Toko | 'baru' | null>(null)

  const toko = useQuery({
    queryKey: ['outlets'],
    queryFn: () => pengaturanApi.daftarToko(),
  })

  const daftar = toko.data?.data ?? []

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Toko & Cabang</h1>
        <Tombol onClick={() => setFormUntuk('baru')}>
          <Plus className="h-5 w-5" aria-hidden />
          Tambah Cabang
        </Tombol>
      </header>

      {toko.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((t) => (
            <li key={t.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 font-semibold text-teks-utama">
                    <Store className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
                    {t.name}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {t.address || 'Tanpa alamat'} · tutup buku {t.business_day_start}
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
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        address: alamat.trim() || undefined,
        phone: hp.trim() || undefined,
        timezone: zona,
        business_day_start: jamTutup,
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
          className="flex flex-col gap-4"
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

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim()}>
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
