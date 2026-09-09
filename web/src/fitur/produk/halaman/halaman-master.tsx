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
import { cn } from '@/bersama/util/cn'
import { produkApi } from '../api'

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
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Kategori, Satuan & Pemasok</h1>

      <div className="flex gap-2" role="tablist">
        {(Object.keys(JUDUL) as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            role="tab"
            aria-selected={tab === t}
            onClick={() => {
              setTab(t)
              setNama('')
              setGalat(null)
            }}
            className={cn(
              'h-10 flex-1 rounded-full border px-4 text-label font-medium',
              tab === t
                ? 'border-utama bg-sorot text-utama'
                : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
            )}
          >
            {JUDUL[t].label}
          </button>
        ))}
      </div>

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
                className="-mr-2 shrink-0 rounded-kontrol p-2 text-bahaya-teks hover:bg-red-50"
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
