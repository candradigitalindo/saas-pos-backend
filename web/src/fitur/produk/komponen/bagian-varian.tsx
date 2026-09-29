import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronRight, Plus } from 'lucide-react'
import type { VarianProduk } from '@/bersama/tipe/katalog'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { formatRupiah } from '@/bersama/util/uang'
import { produkApi } from '../api'

/**
 * Varian = pilihan dengan harga sendiri (ukuran, es/panas, level pedas).
 *
 * Yang disimpan server adalah SELISIH terhadap harga jual, tapi pemilik
 * warung berpikir dalam harga jadi ("Besar 20 ribu") — jadi kolomnya harga
 * jadi, dan selisihnya disebutkan di bawahnya. Stok tetap dihitung per barang.
 */
export function BagianVarian({ produkId, hargaJual }: { produkId: string; hargaJual: number }) {
  const daftar = useQuery({ queryKey: ['varian', produkId], queryFn: () => produkApi.varian(produkId) })
  // undefined = tertutup, null = tambah baru, objek = ubah.
  const [dialog, setDialog] = useState<VarianProduk | null | undefined>(undefined)
  const varian = daftar.data ?? []

  return (
    <Kartu className="flex flex-col gap-3 p-4">
      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">Varian</h2>
          <Tombol jenis="kedua" ukuran="padat" onClick={() => setDialog(null)}>
            <Plus className="h-4 w-4" aria-hidden />
            Tambah
          </Tombol>
        </div>
        <p className="text-keterangan text-teks-redup">
          Pilihan dengan harga sendiri — ukuran, es/panas, level pedas. Kasir wajib memilih satu; stok tetap
          dihitung per barang.
        </p>
      </div>

      {daftar.isLoading ? (
        <KerangkaBaris jumlah={2} />
      ) : varian.length === 0 ? (
        <p className="rounded-kontrol bg-permukaan-2 px-3 py-2 text-label text-teks-redup">Belum ada varian.</p>
      ) : (
        <ul className="-mx-2 flex flex-col">
          {varian.map((v) => (
            <li key={v.id}>
              <button
                type="button"
                onClick={() => setDialog(v)}
                className="flex min-h-12 w-full items-center gap-3 rounded-kontrol px-2 py-1.5 text-left hover:bg-permukaan-2"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-label font-medium text-teks-utama">{v.name}</span>
                  {(v.sku || v.barcode || !v.is_active) && (
                    <span className="block truncate text-keterangan text-teks-redup">
                      {[v.is_active ? null : 'Tidak tampil di kasir', v.sku, v.barcode].filter(Boolean).join(' · ')}
                    </span>
                  )}
                </span>
                <span className="shrink-0 text-label font-semibold tabular-nums text-teks-utama">
                  {formatRupiah(hargaJual + v.price_delta)}
                </span>
                <ChevronRight className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}

      {dialog !== undefined && (
        <DialogVarian produkId={produkId} hargaJual={hargaJual} varian={dialog} onTutup={() => setDialog(undefined)} />
      )}
    </Kartu>
  )
}

function DialogVarian({
  produkId,
  hargaJual,
  varian,
  onTutup,
}: {
  produkId: string
  hargaJual: number
  varian: VarianProduk | null
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState(varian?.name ?? '')
  const [harga, setHarga] = useState(hargaJual + (varian?.price_delta ?? 0))
  const [sku, setSku] = useState(varian?.sku ?? '')
  const [barcode, setBarcode] = useState(varian?.barcode ?? '')
  const [aktif, setAktif] = useState(varian?.is_active ?? true)
  const [yakinHapus, setYakinHapus] = useState(false)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)
  const selisih = harga - hargaJual

  const selesai = (pesan: string) => {
    qc.invalidateQueries({ queryKey: ['varian', produkId] })
    toast.berhasil(pesan)
    onTutup()
  }
  const gagal = (e: unknown) => {
    if (e instanceof GalatAPI) {
      setKolomGalat(e.kolom)
      setGalat(e.pesan)
    } else {
      setGalat('Terjadi kesalahan. Coba lagi.')
    }
  }

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        price_delta: selisih,
        sku: sku.trim(),
        barcode: barcode.trim(),
        is_active: aktif,
      }
      return varian ? produkApi.ubahVarian(produkId, varian.id, isi) : produkApi.buatVarian(produkId, isi)
    },
    onSuccess: (v) => selesai(varian ? `Varian ${v.name} diperbarui.` : `Varian ${v.name} ditambahkan.`),
    onError: gagal,
  })
  const hapus = useMutation({
    mutationFn: () => produkApi.hapusVarian(produkId, varian!.id),
    onSuccess: () => selesai(`Varian ${varian!.name} dihapus.`),
    onError: gagal,
  })
  const sibuk = simpan.isPending || hapus.isPending

  return (
    <Dialog open onOpenChange={(o) => !o && !sibuk && onTutup()}>
      <IsiDialog judul={varian ? `Ubah Varian ${varian.name}` : 'Tambah Varian'}>
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
            label="Nama varian"
            placeholder="Besar"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            maxLength={60}
            galat={galatKolom(kolomGalat, 'name')}
            autoFocus
          />
          <KolomUang
            label="Harga"
            nilai={harga}
            onNilai={setHarga}
            bantuan={
              selisih === 0
                ? 'Sama dengan harga jual barang.'
                : `${selisih > 0 ? '+' : '−'}${formatRupiah(Math.abs(selisih))} dari harga jual — ikut bergeser bila harga jual diubah.`
            }
            galat={galatKolom(kolomGalat, 'price_delta')}
          />
          <div className="grid gap-4 sm:grid-cols-2 sm:items-start">
            <Kolom
              label="Kode (SKU)"
              value={sku}
              onChange={(e) => setSku(e.target.value)}
              maxLength={60}
              bantuan="Boleh kosong."
              galat={galatKolom(kolomGalat, 'sku')}
            />
            <Kolom
              label="Barcode"
              inputMode="numeric"
              value={barcode}
              onChange={(e) => setBarcode(e.target.value)}
              maxLength={60}
              bantuan="Boleh kosong. Dipindai langsung jadi varian ini."
              galat={galatKolom(kolomGalat, 'barcode')}
            />
          </div>
          {varian && (
            <label className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={aktif}
                onChange={(e) => setAktif(e.target.checked)}
                className="h-5 w-5 accent-[var(--warna-utama)]"
              />
              <span className="text-label text-teks-utama">
                Tampil di kasir
                <span className="block text-keterangan text-teks-redup">
                  Matikan sementara bila sedang tidak tersedia.
                </span>
              </span>
            </label>
          )}

          {galat && (
            <p role="alert" className="text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim() || hapus.isPending}>
              {varian ? 'Simpan Varian' : 'Tambah Varian'}
            </Tombol>
            {varian ? (
              // Dua ketukan, bukan dialog "Anda yakin?": penjualan lama tetap
              // utuh, yang hilang hanya pilihannya di kasir.
              <Tombol
                type="button"
                jenis={yakinHapus ? 'bahaya' : 'kedua'}
                memuat={hapus.isPending}
                disabled={simpan.isPending}
                onClick={() => (yakinHapus ? hapus.mutate() : setYakinHapus(true))}
              >
                {yakinHapus ? 'Tekan lagi untuk menghapus' : 'Hapus Varian'}
              </Tombol>
            ) : (
              <Tombol type="button" jenis="kedua" onClick={onTutup} disabled={sibuk}>
                Batal
              </Tombol>
            )}
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
