import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { useSesi } from '@/bersama/hooks/use-sesi'
import type { Shift } from '@/bersama/tipe/pos'
import { kasirApi, type InputCheckout } from './api'

/**
 * Shift yang sedang terbuka di toko aktif.
 *
 * Backend tidak menyediakan filter status, jadi diambil dari halaman pertama
 * daftar shift yang sudah terurut menurun — hanya boleh ada SATU shift terbuka
 * per outlet (dijamin partial unique index), jadi pencarian ini pasti tepat.
 */
export function useShiftAktif() {
  const { tokoAktif } = useSesi()

  const q = useQuery({
    queryKey: ['shift-aktif', tokoAktif],
    queryFn: async () => {
      const hal = await kasirApi.daftarShift(tokoAktif, 1, 20)
      const terbuka = hal.data.find((s) => s.status === 'open')
      if (!terbuka) return null
      // Ambil detailnya supaya rincian kas (penjualan tunai, kas masuk/keluar)
      // ikut terisi — layar tutup shift membutuhkannya.
      return kasirApi.shift(terbuka.id)
    },
    enabled: !!tokoAktif,
    staleTime: 15_000,
  })

  return {
    shift: q.data ?? null,
    memuat: q.isLoading,
    galat: q.error,
    muatUlang: q.refetch,
  }
}

/** Katalog untuk grid kasir, digabung dengan saldo stok toko aktif. */
export function useKatalogKasir(cari: string, kategoriId?: string) {
  const { tokoAktif } = useSesi()

  const produk = useQuery({
    queryKey: ['produk-kasir', cari, kategoriId],
    queryFn: () => kasirApi.produk(cari || undefined, kategoriId),
    staleTime: 60_000,
  })

  const stok = useQuery({
    queryKey: ['stok-kasir', tokoAktif],
    queryFn: () => kasirApi.stok(tokoAktif!),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })

  // Peta stok per produk supaya pencarian di grid tidak O(n²).
  const petaStok = useMemo(() => {
    const m = new Map<string, string>()
    for (const s of stok.data?.data ?? []) m.set(s.product_id, s.qty)
    return m
  }, [stok.data])

  return {
    produk: produk.data?.data ?? [],
    petaStok,
    memuat: produk.isLoading,
    galat: produk.error,
  }
}

/**
 * Checkout.
 *
 * Kunci idempotensi dibuat SEKALI di sini dan dipegang selama percobaan
 * berlangsung. Kalau pengiriman gagal lalu dicoba lagi, kunci yang sama dipakai
 * — server mengenali dan mengembalikan transaksi yang sama, bukan membuat yang
 * kedua.
 */
export function useCheckout() {
  const qc = useQueryClient()

  return useMutation({
    mutationFn: ({ input, kunci }: { input: InputCheckout; kunci: string }) =>
      kasirApi.bayar(input, kunci),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
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
