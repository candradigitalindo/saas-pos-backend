import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, Download, FileUp, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { useSatuan } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { cn } from '@/bersama/util/cn'
import { produkApi, type HasilImpor } from '../api'

const CONTOH_CSV = `name,unit,category,sku,barcode,sell_price,cost_price,min_stock
Kopi Susu,pcs,Minuman,KS-01,,18000,12000,5
Teh Manis,pcs,Minuman,TM-01,,8000,4000,5
`

/**
 * Impor barang dari CSV.
 *
 * Pratinjau (dry-run) WAJIB dijalankan lebih dulu dan tidak bisa dilewati:
 * mengunggah ratusan barang lalu baru tahu setengahnya salah adalah cara
 * tercepat membuat orang berhenti memakai aplikasi.
 */
export function HalamanImpor() {
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()
  const satuan = useSatuan()

  const [csv, setCsv] = useState('')
  const [namaBerkas, setNamaBerkas] = useState('')
  const [pratinjau, setPratinjau] = useState<HasilImpor | null>(null)
  const [galat, setGalat] = useState<string | null>(null)
  const inputBerkas = useRef<HTMLInputElement>(null)

  const jalankan = useMutation({
    mutationFn: ({ dryRun }: { dryRun: boolean }) => produkApi.impor(csv, dryRun),
    onSuccess: (hasil) => {
      if (hasil.dry_run) {
        setPratinjau(hasil)
        return
      }
      qc.invalidateQueries({ queryKey: ['produk'] })
      qc.invalidateQueries({ queryKey: ['katalog-produk'] })
      toast.berhasil(`${hasil.imported} barang berhasil ditambahkan.`)
      navigate('/barang')
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Berkas tidak bisa dibaca.'),
  })

  async function pilihBerkas(e: React.ChangeEvent<HTMLInputElement>) {
    const berkas = e.target.files?.[0]
    if (!berkas) return
    setGalat(null)
    setPratinjau(null)
    setNamaBerkas(berkas.name)
    setCsv(await berkas.text())
  }

  function unduhContoh() {
    const url = URL.createObjectURL(new Blob([CONTOH_CSV], { type: 'text/csv' }))
    const a = document.createElement('a')
    a.href = url
    a.download = 'contoh-barang.csv'
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">Impor Barang</h1>
        <p className="text-isi text-teks-sekunder">
          Tambahkan banyak barang sekaligus dari berkas Excel yang disimpan
          sebagai CSV.
        </p>
      </header>

      <Kartu className="flex flex-col gap-3 p-4">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">
          Isi berkasnya harus begini
        </h2>
        <p className="text-label text-teks-sekunder">
          Baris pertama adalah nama kolom. Yang <strong>wajib</strong> ada:{' '}
          <code className="rounded bg-permukaan-2 px-1">name</code> dan{' '}
          <code className="rounded bg-permukaan-2 px-1">unit</code>. Selebihnya
          boleh dikosongkan: category, sku, barcode, sell_price, cost_price,
          min_stock.
        </p>
        <p className="text-label text-teks-sekunder">
          Satuan dan kategori dikenali dari <strong>namanya</strong>, jadi harus
          sudah dibuat lebih dulu.
          {satuan.data && satuan.data.data.length > 0 && (
            <> Satuan yang tersedia: {satuan.data.data.map((s) => s.name).join(', ')}.</>
          )}
        </p>
        <Tombol jenis="kedua" ukuran="padat" onClick={unduhContoh} className="self-start">
          <Download className="h-5 w-5" aria-hidden />
          Unduh contoh berkas
        </Tombol>
      </Kartu>

      <Kartu className="flex flex-col gap-3 p-4">
        <input
          ref={inputBerkas}
          type="file"
          accept=".csv,text/csv"
          onChange={pilihBerkas}
          className="sr-only"
          id="berkas-csv"
        />
        <label
          htmlFor="berkas-csv"
          className={cn(
            'flex min-h-32 cursor-pointer flex-col items-center justify-center gap-2',
            'rounded-kartu border-2 border-dashed border-garis p-6 text-center',
            'hover:bg-permukaan-2',
          )}
        >
          <FileUp className="h-8 w-8 text-teks-redup" aria-hidden />
          <span className="text-isi font-medium text-teks-utama">
            {namaBerkas || 'Pilih berkas CSV'}
          </span>
          <span className="text-keterangan text-teks-redup">
            {namaBerkas ? 'Ketuk untuk mengganti berkas' : 'Ketuk untuk memilih dari perangkat'}
          </span>
        </label>

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        {csv && !pratinjau && (
          <Tombol
            lebarPenuh
            memuat={jalankan.isPending}
            labelMemuat="Memeriksa berkas…"
            onClick={() => jalankan.mutate({ dryRun: true })}
          >
            Periksa Dulu
          </Tombol>
        )}
      </Kartu>

      {pratinjau && (
        <Kartu className="flex flex-col gap-3 p-4">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Hasil pemeriksaan
          </h2>

          <div className="flex flex-col gap-2">
            <p className="flex items-center gap-2 text-isi text-hijau-800">
              <CheckCircle2 className="h-5 w-5" aria-hidden />
              <strong className="tabular-nums">{pratinjau.imported}</strong> barang siap
              ditambahkan
            </p>
            {pratinjau.failed > 0 && (
              <p className="flex items-center gap-2 text-isi text-jingga-700">
                <TriangleAlert className="h-5 w-5" aria-hidden />
                <strong className="tabular-nums">{pratinjau.failed}</strong> baris
                dilewati karena ada yang salah
              </p>
            )}
          </div>

          {pratinjau.errors.length > 0 && (
            <div className="max-h-64 overflow-y-auto rounded-kontrol border border-garis">
              <table className="w-full text-label">
                <thead className="sticky top-0 bg-permukaan-2">
                  <tr className="text-left text-teks-sekunder">
                    <th scope="col" className="p-2 font-medium">Baris</th>
                    <th scope="col" className="p-2 font-medium">Kolom</th>
                    <th scope="col" className="p-2 font-medium">Masalahnya</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-garis">
                  {pratinjau.errors.map((g, i) => (
                    <tr key={`${g.row}-${g.field}-${i}`}>
                      <td className="p-2 tabular-nums text-teks-utama">{g.row}</td>
                      <td className="p-2 text-teks-sekunder">{g.field || '—'}</td>
                      <td className="p-2 text-teks-utama">{g.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {pratinjau.failed > 0 && (
            <p className="text-label text-teks-sekunder">
              Baris yang salah akan dilewati — yang benar tetap masuk. Anda juga
              bisa memperbaiki berkasnya dulu lalu memeriksa ulang.
            </p>
          )}

          <div className="flex flex-col gap-2 sm:flex-row">
            <Tombol
              lebarPenuh
              memuat={jalankan.isPending}
              labelMemuat="Menambahkan barang…"
              disabled={pratinjau.imported === 0}
              onClick={() => jalankan.mutate({ dryRun: false })}
            >
              Tambahkan {pratinjau.imported} Barang
            </Tombol>
            <Tombol
              jenis="kedua"
              lebarPenuh
              onClick={() => {
                setPratinjau(null)
                setCsv('')
                setNamaBerkas('')
                if (inputBerkas.current) inputBerkas.current.value = ''
              }}
            >
              Ganti Berkas
            </Tombol>
          </div>
        </Kartu>
      )}
    </div>
  )
}
