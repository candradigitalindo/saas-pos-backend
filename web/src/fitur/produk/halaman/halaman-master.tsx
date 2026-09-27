import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useKategori, usePemasok, useSatuan } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { produkApi } from '../api'
import { SegmenPilihan } from '@/bersama/ui/segmen'

type Tab = 'kategori' | 'satuan' | 'pemasok'

const JUDUL: Record<Tab, { label: string; tunggal: string; contoh: string; bantuan: string }> = {
  kategori: {
    label: 'Kategori',
    tunggal: 'kategori',
    contoh: 'Minuman',
    bantuan: 'Mengelompokkan barang supaya gampang dicari di kasir.',
  },
  satuan: {
    label: 'Satuan',
    tunggal: 'satuan',
    contoh: 'pcs',
    bantuan: 'Satuan jualan: pcs, kg, botol, bungkus.',
  },
  pemasok: {
    label: 'Pemasok',
    tunggal: 'pemasok',
    contoh: 'Toko Grosir Jaya',
    bantuan: 'Tempat Anda kulakan barang.',
  },
}

/** Kategori, satuan, dan pemasok — data pendukung yang jarang diubah. */
export function HalamanMaster() {
  const [tab, setTab] = useState<Tab>('kategori')
  const toast = useToast()
  const qc = useQueryClient()

  const kategori = useKategori()
  const satuan = useSatuan()
  const pemasok = usePemasok()

  const [nama, setNama] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const sumber =
    tab === 'kategori' ? kategori : tab === 'satuan' ? satuan : pemasok
  const daftar = sumber.data?.data ?? []

  const kunciCache =
    tab === 'kategori' ? 'katalog-kategori' : tab === 'satuan' ? 'katalog-satuan' : 'katalog-pemasok'

  // Ketiganya dibuat lewat endpoint berbeda tapi yang dipakai layar ini cuma
  // id dan nama, jadi hasilnya disamakan ke bentuk itu.
  const tambah = useMutation<{ id: string; name: string }>({
    mutationFn: () => {
      const n = nama.trim()
      if (tab === 'kategori') return produkApi.buatKategori(n)
      if (tab === 'satuan') return produkApi.buatSatuan(n)
      return produkApi.buatPemasok({ name: n })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [kunciCache] })
      toast.berhasil(`${JUDUL[tab].label} "${nama.trim()}" ditambahkan.`)
      setNama('')
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  const hapus = useMutation<null, Error, string>({
    mutationFn: (id: string) => {
      if (tab === 'kategori') return produkApi.hapusKategori(id)
      if (tab === 'satuan') return produkApi.hapusSatuan(id)
      return produkApi.hapusPemasok(id)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [kunciCache] })
      toast.berhasil('Dihapus.')
    },
    onError: (e) =>
      toast.gagal(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Masih dipakai oleh barang lain, jadi belum bisa dihapus.'
            : e.pesan
          : 'Gagal menghapus.',
      ),
  })

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Kategori, Satuan & Pemasok</h1>

      {/* `aria-pressed` pada satu grup, bukan role="tab". Markup sebelumnya
          memakai role="tablist"/"tab" tanpa `tabpanel` maupun `aria-controls`
          pasangannya — ARIA setengah jadi yang menjanjikan ke pembaca layar
          sesuatu yang tidak ada di halaman. */}
      <SegmenPilihan
        label="Jenis data induk"
        nilai={tab}
        onPilih={(t) => {
          setTab(t)
          setNama('')
          setGalat(null)
        }}
        pilihan={(Object.keys(JUDUL) as Tab[]).map((t) => [t, JUDUL[t].label] as const)}
      />

      <Kartu className="p-4">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            tambah.mutate()
          }}
          className="flex flex-col gap-3"
        >
          <Kolom
            label={`Nama ${JUDUL[tab].tunggal}`}
            placeholder={JUDUL[tab].contoh}
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            bantuan={JUDUL[tab].bantuan}
            galat={galat ?? undefined}
          />
          <Tombol type="submit" memuat={tambah.isPending} disabled={!nama.trim()}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah {JUDUL[tab].label}
          </Tombol>
        </form>
      </Kartu>

      {sumber.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <p className="text-center text-isi text-teks-redup">
          Belum ada {JUDUL[tab].tunggal}. Tambahkan yang pertama di atas.
        </p>
      ) : (
        <Kartu className="divide-y divide-garis">
          {daftar.map((x) => (
            <div key={x.id} className="flex min-h-14 items-center justify-between gap-3 px-4">
              <span className="min-w-0 truncate text-isi text-teks-utama">{x.name}</span>
              <button
                type="button"
                onClick={() => hapus.mutate(x.id)}
                aria-label={`Hapus ${x.name}`}
                // 48px untuk jari, menyusut ke 40px pada penunjuk halus —
                // aturan yang sama dengan tombol "padat" (ui/01 §3). Sebelumnya
                // p-2 di sekeliling ikon 20px hanya menghasilkan 36px.
                className="-mr-2 flex h-12 w-12 shrink-0 items-center justify-center rounded-kontrol text-bahaya-teks hover:bg-bahaya-teks/10 pointer-fine:h-10 pointer-fine:w-10"
              >
                <Trash2 className="h-5 w-5" aria-hidden />
              </button>
            </div>
          ))}
        </Kartu>
      )}
    </div>
  )
}
