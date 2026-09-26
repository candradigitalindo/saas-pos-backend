import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { antrekan } from '@/lib/offline/antrean'
import { ingat, ingatan } from '@/lib/offline/ingatan'
import { kurangiStokLokal, produkLokal, useKatalogLokal } from '@/lib/offline/katalog-lokal'
import { pratinjauBaris } from '@/bersama/util/uang'
import type { ItemTransaksi, Shift, Transaksi } from '@/bersama/tipe/pos'
import { kasirApi, type InputCheckout } from './api'
import { hitungTotal, type AturanHarga, type BarisHitung } from './total'

/**
 * Shift yang sedang terbuka di toko aktif.
 *
 * Backend tidak menyediakan filter status, jadi diambil dari halaman pertama
 * daftar shift yang sudah terurut menurun — hanya boleh ada SATU shift terbuka
 * per outlet (dijamin partial unique index), jadi pencarian ini pasti tepat.
 */
export function useShiftAktif() {
  const { tokoAktif } = useSesi()

  const kunciIngat = `shift-aktif.${tokoAktif}`
  const q = useQuery({
    queryKey: ['shift-aktif', tokoAktif],
    queryFn: async () => {
      const hal = await kasirApi.daftarShift(tokoAktif, 1, 20)
      const terbuka = hal.data.find((s) => s.status === 'open')
      // Ambil detailnya supaya rincian kas (penjualan tunai, kas masuk/keluar)
      // ikut terisi — layar tutup shift membutuhkannya.
      const hasil = terbuka ? await kasirApi.shift(terbuka.id) : null
      ingat(kunciIngat, hasil)
      return hasil
    },
    enabled: !!tokoAktif,
    // Shift terakhir yang diketahui dipakai sebagai data awal: tanpa ini kasir
    // yang membuka ulang aplikasi tanpa sinyal disodori "Buka shift" (yang juga
    // butuh internet) dan tidak bisa berjualan. Transaksi offline tetap membawa
    // shift_id; bila shift itu ternyata sudah ditutup di perangkat lain,
    // server menolaknya saat sinkron dan transaksinya masuk "perlu diperiksa".
    initialData: () => (tokoAktif ? ingatan<Shift | null>(kunciIngat) : undefined),
    initialDataUpdatedAt: 0,
    staleTime: 15_000,
  })

  return {
    shift: q.data ?? null,
    memuat: q.isLoading,
    galat: q.error,
    muatUlang: q.refetch,
  }
}

/**
 * Katalog untuk grid kasir — dibaca dari penyimpanan lokal, bukan dari server.
 *
 * Kasir TIDAK PERNAH diblokir status koneksi, dan pencarian harus di bawah
 * 100 ms; keduanya hanya mungkin bila sumbernya lokal. Mesin sinkronisasi yang
 * menyegarkan Dexie di belakang layar.
 */
export function useKatalogKasir(cari: string, kategoriId?: string) {
  const { tokoAktif } = useSesi()
  return useKatalogLokal(cari, tokoAktif, kategoriId)
}

/**
 * Checkout.
 *
 * Kunci idempotensi dibuat SEKALI di layar bayar dan dipegang selama percobaan
 * berlangsung — server mengenali kunci yang sama dan mengembalikan transaksi
 * yang itu juga, bukan membuat yang kedua.
 *
 * Bila jaringan mati, transaksi TIDAK gagal: ia masuk antrean lokal dan struknya
 * tetap tampil dengan keterangan "menunggu dikirim". Kasir tidak pernah
 * diblokir oleh status koneksi.
 */
export function useCheckout() {
  const qc = useQueryClient()

  return useMutation<
    HasilCheckout,
    Error,
    { input: InputCheckout; kunci: string; aturan?: AturanHarga }
  >({
    mutationFn: async ({ input, kunci, aturan }) => {
      try {
        const transaksi = await kasirApi.bayar(input, kunci)
        return { transaksi, diantre: false }
      } catch (e) {
        // Hanya kegagalan yang MEMANG bisa diantre yang boleh jadi transaksi
        // offline. Penolakan seperti 422 (stok tidak ada, pelanggan wajib untuk
        // kasbon) harus tetap muncul ke kasir sekarang juga — mengantrekannya
        // hanya menunda kabar buruk yang sama.
        if (!(e instanceof GalatAPI) || !e.bisaDiantre) throw e

        const transaksi = await simpanKeAntrean(input, kunci, aturan)
        return { transaksi, diantre: true }
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
}

export interface HasilCheckout {
  transaksi: Transaksi
  /** true bila transaksi baru masuk antrean, belum sampai ke server. */
  diantre: boolean
}

/**
 * Menyusun struk sementara dari data lokal dan memasukkan operasinya ke antrean.
 *
 * Angka di struk ini PERKIRAAN — dihitung dari harga di katalog lokal. Struk
 * yang sah tetap yang dari server, dan akan menggantikannya begitu terkirim.
 * Karena itu layarnya diberi lencana "menunggu dikirim", bukan "tersimpan".
 */
async function simpanKeAntrean(
  input: InputCheckout,
  kunci: string,
  aturan?: AturanHarga,
): Promise<Transaksi> {
  const idTransaksi = input.id ?? kunci

  const items: ItemTransaksi[] = []
  const untukHitung: BarisHitung[] = []
  for (const [i, it] of input.items.entries()) {
    const p = await produkLokal(it.product_id)
    const hargaSatuan = p?.sell_price ?? 0
    const lineTotal = pratinjauBaris(hargaSatuan, it.qty, it.discount_amount ?? 0)
    untukHitung.push({ harga: hargaSatuan, qty: it.qty, diskon: it.discount_amount ?? 0 })
    items.push({
      id: `${idTransaksi}-${i}`,
      product_id: it.product_id,
      product_name: p?.name ?? 'Barang',
      unit_name: p?.unit_name ?? '',
      qty: it.qty,
      unit_price: hargaSatuan,
      unit_cost: p?.cost_price ?? 0,
      discount_amount: it.discount_amount ?? 0,
      tax_amount: 0,
      line_total: lineTotal,
    })
  }

  const dibayar = input.payments.reduce((j, p) => j + p.amount, 0)
  // Rumus yang sama dengan server (pajak, biaya layanan, diskon transaksi) —
  // lihat total.ts.
  const r = hitungTotal(untukHitung, aturan, input.order_discount ?? 0)
  const total = r.total
  const sekarang = new Date().toISOString()

  await antrekan({
    id: idTransaksi,
    op: 'sale.create',
    payload: { ...input, id: idTransaksi, client_created_at: sekarang },
    ringkasan: items.map((i) => `${i.qty} ${i.product_name}`).join(', ') || 'Transaksi',
    nominal: total,
  })

  // Stok lokal ikut turun supaya kasir tidak melihat angka yang jelas basi
  // setelah ia sendiri baru saja menjualnya.
  await kurangiStokLokal(input.outlet_id, input.items.map((i) => ({
    productId: i.product_id,
    qty: i.qty,
  })))

  return {
    id: idTransaksi,
    outlet_id: input.outlet_id,
    shift_id: input.shift_id,
    customer_id: input.customer_id,
    // Nomor struk sungguhan dibuat server. Sampai terkirim, kasir melihat
    // penanda sementara yang jelas-jelas bukan nomor resmi.
    receipt_no: 'Belum bernomor',
    order_type: input.order_type ?? 'takeaway',
    status: 'completed',
    subtotal: r.subtotal,
    discount_amount: r.discount_amount,
    tax_amount: r.tax_amount,
    service_amount: r.service_amount,
    rounding_amount: 0,
    total,
    paid_amount: dibayar,
    change_amount: Math.max(0, dibayar - total),
    cost_total: 0,
    gross_profit: 0,
    occurred_at: sekarang,
    business_date: '',
    items,
    payments: input.payments.map((p, i) => ({
      id: `${idTransaksi}-b${i}`,
      method: p.method,
      amount: p.amount,
      fee_amount: 0,
      paid_at: sekarang,
    })),
    created_at: sekarang,
  }
}

/** Kunci idempotensi baru. Dipanggil sekali saat layar bayar dibuka. */
export function kunciBaru(): string {
  return ulid()
}

export function useBukaShift() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      outletId,
      modalAwal,
      catatan,
    }: {
      outletId: string
      modalAwal: number
      catatan?: string
    }) => kasirApi.bukaShift(outletId, modalAwal, catatan),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shift-aktif'] }),
  })
}

export function useTutupShift() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      id,
      uangDihitung,
      catatan,
    }: {
      id: string
      uangDihitung: number
      catatan?: string
    }) => kasirApi.tutupShift(id, uangDihitung, catatan),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shift-aktif'] }),
  })
}

export function useSerahTerimaShift() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      id,
      uangDihitung,
      modalDitinggal,
      catatan,
    }: {
      id: string
      uangDihitung: number
      modalDitinggal?: number
      catatan?: string
    }) => kasirApi.serahTerimaShift(id, uangDihitung, modalDitinggal, catatan),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shift-aktif'] }),
  })
}

export function useRiwayatTransaksi(filter: {
  outlet_id?: string
  business_date?: string
  status?: string
  shift_id?: string
}) {
  return useQuery({
    queryKey: ['riwayat-transaksi', filter],
    queryFn: () => kasirApi.daftarTransaksi(filter),
    staleTime: 15_000,
  })
}

export function useGerakanKas(shiftId?: string) {
  const qc = useQueryClient()

  const daftar = useQuery({
    queryKey: ['gerakan-kas', shiftId],
    queryFn: () => kasirApi.daftarKas(shiftId),
    enabled: !!shiftId,
  })

  const catat = useMutation({
    mutationFn: kasirApi.catatKas,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gerakan-kas'] })
      // Kas masuk/keluar mengubah "uang yang seharusnya ada di laci".
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
    },
  })

  return { daftar, catat }
}

/** Ringkasan shift untuk layar tutup — semuanya dari server, tanpa hitungan sendiri. */
export function rincianShift(shift: Shift) {
  const modalAwal = shift.opening_cash
  const penjualanTunai = shift.cash_sales ?? 0
  const kasMasuk = shift.cash_in ?? 0
  const kasKeluar = shift.cash_out ?? 0
  return {
    modalAwal,
    penjualanTunai,
    kasMasuk,
    kasKeluar,
    seharusnya: shift.expected_cash,
  }
}
