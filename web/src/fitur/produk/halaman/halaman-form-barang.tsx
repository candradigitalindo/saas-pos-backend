import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp, ScanLine, TrendingUp, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { PemilihFoto } from '@/bersama/komponen/pemilih-foto'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useDaftarHarga, useKategori, useSatuan } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { PemindaiBarcode } from '@/bersama/komponen/pemindai-barcode'
import { bisaMemindai } from '@/bersama/hooks/use-pemindai'
import { produkApi } from '../api'
import { BagianVarian } from '../komponen/bagian-varian'
import { IsianGrosir, siapkanGrosir } from '../komponen/isian-grosir'
import { IsianHargaKhusus } from '../komponen/isian-harga-khusus'
import type { TingkatGrosir } from '@/bersama/tipe/katalog'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/**
 * Tambah / ubah barang.
 *
 * Form panjang lebih dari tujuh kolom bikin orang menyerah, jadi yang tampil
 * hanya tiga yang benar-benar menentukan: nama, harga jual, harga beli. Sisanya
 * — SKU, barcode, kategori, stok minimum — bersembunyi di "Detail lainnya"
 * yang tertutup (ui/05-ALUR-UTAMA.md §1).
 */
export function HalamanFormBarang() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const onboarding = params.get('onboarding') === '1'
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const satuan = useSatuan()
  const kategori = useKategori()
  const sedangUbah = !!id

  // Foto barang BARU tidak bisa langsung diunggah: endpointnya butuh id, dan
  // id baru lahir setelah barangnya tersimpan. Jadi fotonya ditahan di sini
  // lalu dikirim tepat setelah penyimpanan berhasil.
  const [fotoTertunda, setFotoTertunda] = useState<Blob | null>(null)
  const [fotoURL, setFotoURL] = useState<string | null>(null)
  const [sedangUnggah, setSedangUnggah] = useState(false)

  const detail = useQuery({
    queryKey: ['produk', id],
    queryFn: () => produkApi.satu(id!),
    enabled: sedangUbah,
  })

  const [form, setForm] = useState({
    name: '',
    sell_price: 0,
    cost_price: 0,
    unit_id: '',
    category_id: '',
    sku: '',
    barcode: '',
    min_stock: '',
    track_stock: true,
    description: '',
  })
  const [grosir, setGrosir] = useState<TingkatGrosir[]>([])
  const [khusus, setKhusus] = useState<Record<string, number>>({})
  const daftarHarga = useDaftarHarga()
  const [bukaDetail, setBukaDetail] = useState(false)
  const [bukaPindai, setBukaPindai] = useState(false)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  // Isi form dari data yang ada saat mengubah.
  useEffect(() => {
    const p = detail.data
    if (!p) return
    setForm({
      name: p.name,
      sell_price: p.sell_price,
      cost_price: p.cost_price,
      unit_id: p.unit_id,
      category_id: p.category_id ?? '',
      sku: p.sku ?? '',
      barcode: p.barcode ?? '',
      min_stock: p.min_stock === '0' ? '' : p.min_stock,
      track_stock: p.track_stock,
      description: p.description ?? '',
    })
    setGrosir(p.wholesale_prices ?? [])
    setKhusus(Object.fromEntries((p.special_prices ?? []).map((s) => [s.price_list_id, s.price])))
    setFotoURL(p.image_url || null)
  }, [detail.data])

  // Satuan pertama dipilih otomatis: pemilik warung tidak memikirkan "satuan"
  // saat menambah barang pertamanya, dan pilihan kosong akan menghentikannya.
  useEffect(() => {
    const pertama = satuan.data?.data[0]
    if (!form.unit_id && pertama) setForm((f) => ({ ...f, unit_id: pertama.id }))
  }, [satuan.data, form.unit_id])

  const simpan = useMutation({
    mutationFn: () => {
      const g = siapkanGrosir(grosir)
      if ('galat' in g) return Promise.reject(new GalatAPI(400, g.galat))
      const isi = {
        wholesale_prices: g.tingkat,
        // Hanya daftar yang masih ada; yang dikosongkan tidak dikirim (= dihapus).
        // Selama daftar harga belum termuat kolom ini TIDAK dikirim — "[]"
        // akan menghapus harga khusus yang sudah ada.
        ...(daftarHarga.data
          ? {
              special_prices: daftarHarga.data
                .filter((d) => (khusus[d.id] ?? 0) > 0)
                .map((d) => ({ price_list_id: d.id, price: khusus[d.id]! })),
            }
          : {}),
        name: form.name.trim(),
        unit_id: form.unit_id,
        sell_price: form.sell_price,
        cost_price: form.cost_price,
        category_id: form.category_id || undefined,
        sku: form.sku.trim() || undefined,
        barcode: form.barcode.trim() || undefined,
        min_stock: form.min_stock.trim() || undefined,
        track_stock: form.track_stock,
        description: form.description.trim(),
      }
      return sedangUbah ? produkApi.ubah(id!, isi) : produkApi.buat(isi)
    },
    onSuccess: async (p) => {
      if (fotoTertunda) {
        setSedangUnggah(true)
        try {
          await produkApi.unggahFoto(p.id, fotoTertunda)
          setFotoTertunda(null)
        } catch {
          // Barangnya SUDAH tersimpan; fotonya yang gagal. Dikabarkan apa
          // adanya, bukan dijadikan kegagalan seluruh penyimpanan — memaksa
          // pengguna mengetik ulang semua kolom karena satu foto adalah
          // hukuman yang tidak sepadan.
          toast.tampilkan('Barang tersimpan, tapi fotonya gagal diunggah.', 'perhatian')
        } finally {
          setSedangUnggah(false)
        }
      }
      qc.invalidateQueries({ queryKey: ['produk'] })
      qc.invalidateQueries({ queryKey: ['katalog-produk'] })
      toast.berhasil(sedangUbah ? `${p.name} diperbarui.` : `${p.name} ditambahkan.`)
      // Saat onboarding, langkah berikutnya adalah mengisi stok awal barang ini.
      navigate(onboarding ? `/stok/koreksi?product_id=${p.id}&onboarding=1` : '/barang')
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        // 422 dari validasi kolom tampil di kolomnya; 422 dari aturan layanan
        // (mis. tingkat grosir) hanya membawa `request` — tanpa ini pesannya hilang.
        setGalat(e.status === 422 ? (e.kolom.request ?? null) : e.pesan)
        // Kolom tersembunyi yang bermasalah harus ikut terlihat, kalau tidak
        // pengguna melihat form yang menolak simpan tanpa alasan yang tampak.
        if (['sku', 'barcode', 'min_stock', 'category_id'].some((k) => e.kolom[k])) {
          setBukaDetail(true)
        }
      } else {
        setGalat('Terjadi kesalahan. Coba lagi.')
      }
    },
  })

  if (sedangUbah && detail.isLoading) return <KerangkaBaris jumlah={4} />

  const belumAdaSatuan = !satuan.isLoading && (satuan.data?.data.length ?? 0) === 0

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">
        {sedangUbah ? 'Ubah Barang' : 'Tambah Barang'}
      </h1>

      {belumAdaSatuan && (
        <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
          Belum ada satuan (pcs, kg, botol). Buat dulu di Pengaturan &rsaquo; Satuan.
        </p>
      )}

      <Kartu className="p-4">
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
          <PemilihFoto
            nama={form.name}
            url={fotoURL ?? (fotoTertunda ? URL.createObjectURL(fotoTertunda) : null)}
            sedangUnggah={sedangUnggah}
            onPilih={async (berkas) => {
              if (!sedangUbah) { setFotoTertunda(berkas); return }
              setSedangUnggah(true)
              try {
                const { image_url } = await produkApi.unggahFoto(id!, berkas)
                setFotoURL(image_url)
                qc.invalidateQueries({ queryKey: ['katalog-produk'] })
                toast.berhasil('Foto tersimpan.')
              } catch {
                toast.tampilkan('Foto gagal diunggah. Coba lagi.', 'perhatian')
              } finally {
                setSedangUnggah(false)
              }
            }}
            onHapus={async () => {
              if (!sedangUbah) { setFotoTertunda(null); return }
              setSedangUnggah(true)
              try {
                await produkApi.hapusFoto(id!)
                setFotoURL(null)
                qc.invalidateQueries({ queryKey: ['katalog-produk'] })
              } finally {
                setSedangUnggah(false)
              }
            }}
          />

          <Kolom
            label="Nama barang"
            placeholder="Kopi Susu"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            galat={galatKolom(kolomGalat, 'name')}
            autoFocus
            required
          />

          <div className="grid gap-4 sm:grid-cols-2 sm:items-start">
            <KolomUang
              label="Harga jual"
              nilai={form.sell_price}
              onNilai={(n) => setForm((f) => ({ ...f, sell_price: n }))}
              bantuan="Harga yang dibayar pembeli."
              galat={galatKolom(kolomGalat, 'sell_price')}
            />

            <KolomUang
              label="Harga beli"
              nilai={form.cost_price}
              onNilai={(n) => setForm((f) => ({ ...f, cost_price: n }))}
              bantuan="Modal per satuan. Dipakai menghitung untung."
              galat={galatKolom(kolomGalat, 'cost_price')}
            />
          </div>

          <InfoUntung jual={form.sell_price} modal={form.cost_price} />

          <IsianGrosir
            nilai={grosir}
            onNilai={(v) => {
              setGrosir(v)
              setGalat(null)
            }}
            hargaJual={form.sell_price}
            hargaBeli={form.cost_price}
            satuan={satuan.data?.data.find((s) => s.id === form.unit_id)?.name}
          />

          <IsianHargaKhusus
            daftar={daftarHarga.data ?? []}
            nilai={khusus}
            onNilai={setKhusus}
            hargaBeli={form.cost_price}
          />

          <button
            type="button"
            onClick={() => setBukaDetail((v) => !v)}
            aria-expanded={bukaDetail}
            className="-mx-2 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot"
          >
            {bukaDetail ? (
              <ChevronUp className="h-4 w-4" aria-hidden />
            ) : (
              <ChevronDown className="h-4 w-4" aria-hidden />
            )}
            Detail lainnya
          </button>

          {bukaDetail && (
            <div className="grid gap-4 border-l-2 border-garis pl-3 sm:grid-cols-2 sm:items-start">
              <Pilihan
                label="Satuan"
                value={form.unit_id}
                onChange={(e) => setForm((f) => ({ ...f, unit_id: e.target.value }))}
                galat={galatKolom(kolomGalat, 'unit_id')}
              >
                {satuan.data?.data.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Pilihan>

              <Pilihan
                label="Kategori"
                value={form.category_id}
                onChange={(e) => setForm((f) => ({ ...f, category_id: e.target.value }))}
                galat={galatKolom(kolomGalat, 'category_id')}
              >
                <option value="">Tanpa kategori</option>
                {kategori.data?.data.map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.name}
                  </option>
                ))}
              </Pilihan>

              <Kolom
                label="Kode barang (SKU)"
                value={form.sku}
                onChange={(e) => setForm((f) => ({ ...f, sku: e.target.value }))}
                bantuan="Boleh dikosongkan."
                galat={galatKolom(kolomGalat, 'sku')}
              />

              <div className="flex items-end gap-2 sm:col-span-2">
                <div className="min-w-0 flex-1">
                  <Kolom
                    label="Barcode"
                    inputMode="numeric"
                    value={form.barcode}
                    onChange={(e) => setForm((f) => ({ ...f, barcode: e.target.value }))}
                    bantuan="Boleh dikosongkan. Isi supaya barang ini bisa dipindai di kasir."
                    galat={galatKolom(kolomGalat, 'barcode')}
                  />
                </div>
                {/* Tanpa tombol ini, satu-satunya cara mendaftarkan barcode
                    adalah mengetik 13 digit dari kemasan — dan itu yang
                    membuat fitur pindai di kasir tidak pernah terpakai. */}
                {bisaMemindai() && (
                  <Tombol
                    type="button"
                    jenis="kedua"
                    ukuran="ikon"
                    onClick={() => setBukaPindai(true)}
                    aria-label="Pindai barcode barang ini"
                    className="mb-6 shrink-0"
                  >
                    <ScanLine className="h-5 w-5" aria-hidden />
                  </Tombol>
                )}
              </div>

              <Kolom
                label="Ingatkan bila stok tinggal"
                inputMode="decimal"
                value={form.min_stock}
                onChange={(e) => setForm((f) => ({ ...f, min_stock: e.target.value }))}
                bantuan="Barang akan muncul di peringatan stok menipis."
                galat={galatKolom(kolomGalat, 'min_stock')}
              />

              <label className="flex items-center gap-3 sm:col-span-2">
                <input
                  type="checkbox"
                  checked={form.track_stock}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, track_stock: e.target.checked }))
                  }
                  className="h-5 w-5 accent-[var(--warna-utama)]"
                />
                <span className="text-label text-teks-utama">
                  Hitung stoknya
                  <span className="block text-keterangan text-teks-redup">
                    Matikan untuk jasa, atau masakan yang dibuat saat dipesan — di aplikasi antar
                    barang seperti ini selalu tampil tersedia.
                  </span>
                </span>
              </label>

              {/* Hanya untuk menu aplikasi antar; GoFood memotong di 250
                  karakter, jadi batas itulah yang ditunjukkan. */}
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <div className="flex items-baseline justify-between gap-3">
                  <label htmlFor="deskripsi-barang" className="text-label font-medium text-teks-sekunder">
                    Deskripsi
                  </label>
                  <span
                    className={cn(
                      'text-keterangan tabular-nums',
                      form.description.length > 250 ? 'text-jingga-700' : 'text-teks-redup',
                    )}
                  >
                    {form.description.length}/250
                  </span>
                </div>
                <textarea
                  id="deskripsi-barang"
                  rows={3}
                  maxLength={500}
                  value={form.description}
                  onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                  placeholder="mis. Nasi goreng dengan telur mata sapi, ayam suwir, dan kerupuk"
                  aria-describedby="deskripsi-barang-bantuan"
                  className="rounded-kontrol border border-garis bg-permukaan px-3 py-2.5 text-isi text-teks-utama placeholder:text-teks-redup focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-utama"
                />
                <p id="deskripsi-barang-bantuan" className="text-keterangan text-teks-redup">
                  Boleh dikosongkan. Tampil di menu GoFood/GrabFood bila menu dikirim dari POS
                  {form.description.length > 250 ? ' — GoFood hanya menampilkan 250 karakter pertama.' : '.'}
                </p>
              </div>
            </div>
          )}

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <Tombol
            type="submit"
            lebarPenuh
            memuat={simpan.isPending}
            disabled={!form.name.trim() || !form.unit_id}
          >
            {sedangUbah ? 'Simpan Perubahan' : 'Simpan Barang'}
          </Tombol>
          {!sedangUbah && (
            <p className="-mt-2 text-center text-keterangan text-teks-redup">
              Punya ukuran atau level pedas? Varian bisa ditambahkan setelah barang disimpan.
            </p>
          )}
        </form>
      </Kartu>

      {/* Harga varian dihitung dari harga jual yang TERSIMPAN, bukan yang
          sedang diketik — selisihnya disimpan terhadap angka itu. */}
      {sedangUbah && detail.data && <BagianVarian produkId={id!} hargaJual={detail.data.sell_price} />}

      <PemindaiBarcode
        terbuka={bukaPindai}
        onTutup={() => setBukaPindai(false)}
        judul="Pindai Barcode Barang"
        keterangan="Arahkan kamera ke barcode di kemasan. Kodenya langsung terisi."
        onKode={(kode) => {
          setForm((f) => ({ ...f, barcode: kode }))
          setBukaPindai(false)
          return true
        }}
      />
    </div>
  )
}

/**
 * Untung per barang, langsung saat harga diketik — pemilik warung menetapkan
 * harga jual dari modalnya, dan angka ini yang sebenarnya ia hitung di kepala.
 * Harga jual di bawah modal diberi peringatan, bukan ditolak (obral itu sah).
 */
function InfoUntung({ jual, modal }: { jual: number; modal: number }) {
  if (jual <= 0 || modal <= 0) return null
  const untung = jual - modal
  if (untung < 0) {
    return (
      <p className="-mt-1 flex items-start gap-2 rounded-kontrol bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
        <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
        Harga jual di bawah modal — rugi {formatRupiah(-untung)} per barang.
      </p>
    )
  }
  const persen = Math.round((untung / jual) * 100)
  return (
    <p className="-mt-1 flex items-center gap-2 rounded-kontrol bg-sorot px-3 py-2 text-label text-hijau-800">
      <TrendingUp className="h-4 w-4 shrink-0" aria-hidden />
      Untung {formatRupiah(untung)} per barang · margin {persen}%
    </p>
  )
}
