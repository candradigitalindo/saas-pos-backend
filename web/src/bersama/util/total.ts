/**
 * Total belanja, dihitung PERSIS seperti server (services.priceCheckout).
 * Dipakai kasir (keranjang, struk offline) dan contoh hitungan di pengaturan
 * pajak toko — karena itu tinggal di bersama/, bukan di fitur/kasir.
 *
 * Kenapa klien menghitung sendiri, padahal "angka final selalu dari server":
 * saat offline tidak ada server yang bisa ditanya, dan untuk QRIS/kasbon total
 * inilah yang DIBAYAR PAS — server menolak pembayaran yang kurang serupiah
 * pun. Total yang melupakan pajak eksklusif atau biaya layanan membuat setiap
 * pembayaran QRIS/kasbon di outlet berpajak ditolak (dan transaksi offline-nya
 * ditolak saat sinkron).
 *
 * Aturannya disalin dari helpers/money.go + priceCheckout, dan dijaga oleh
 * kasus-total.json yang diuji di DUA sisi (services/total_kasir_test.go dan
 * total.test.ts). Jangan mengubah rumus di sini tanpa mengubah server — dan
 * sebaliknya.
 *
 *   per baris : kotor = bulat(qty × harga); dasar = kotor − diskon baris
 *               pajak mati     → total baris = dasar
 *               pajak inklusif → pajak = dasar − bulat(dasar ÷ (1 + tarif))
 *               pajak eksklusif→ pajak = bulat(dasar × tarif); total baris = dasar + pajak
 *   transaksi : layanan = bulat((Σ kotor − Σ diskon − diskon transaksi) × tarif layanan)
 *               total   = Σ total baris − diskon transaksi + layanan
 *
 * "bulat" = setengah menjauhi nol, seperti shopspring/decimal Round(0).
 */
import Decimal from 'decimal.js'

/** Presisi lebar supaya perkalian tidak pernah terpotong sebelum dibulatkan. */
const D = Decimal.clone({ precision: 40, rounding: Decimal.ROUND_HALF_UP })

/** Presisi pembagian shopspring/decimal (DivisionPrecision). */
const PRESISI_BAGI = 16

/** Pengaturan harga satu cabang, dari GET /me (outlets[]). */
export interface AturanHarga {
  tax_enabled: boolean
  tax_rate: string
  tax_inclusive: boolean
  service_charge_rate: string
}

export interface BarisHitung {
  /** Harga satuan rupiah bulat (sudah termasuk selisih varian). */
  harga: number
  /** Jumlah, string desimal. */
  qty: string
  /** Diskon baris, rupiah bulat. */
  diskon: number
}

/** Nama kolom sama dengan balasan server supaya mudah dibandingkan. */
export interface RincianTotal {
  subtotal: number
  discount_amount: number
  tax_amount: number
  service_amount: number
  total: number
}

function bulat(d: Decimal): number {
  return d.toDecimalPlaces(0, Decimal.ROUND_HALF_UP).toNumber()
}

function tarif(teks: string | undefined): Decimal {
  try {
    return new D(teks || '0')
  } catch {
    return new D(0)
  }
}

/** Pajak yang sudah termasuk di dalam `kotor` (helpers.InclusiveTax). */
function pajakInklusif(kotor: number, t: Decimal): number {
  if (t.isZero()) return 0
  const bersih = new D(kotor).div(t.plus(1)).toDecimalPlaces(PRESISI_BAGI, Decimal.ROUND_HALF_UP)
  return kotor - bulat(bersih)
}

export function hitungTotal(
  baris: BarisHitung[],
  aturan: AturanHarga | undefined,
  diskonTransaksi = 0,
): RincianTotal {
  const tarifPajak = tarif(aturan?.tax_rate)
  const pajakAktif = !!aturan?.tax_enabled && tarifPajak.greaterThan(0)
  const tarifLayanan = tarif(aturan?.service_charge_rate)

  let subtotal = 0
  let diskonBaris = 0
  let pajak = 0
  let total = 0
  for (const b of baris) {
    let qty: Decimal
    try {
      qty = new D(b.qty)
    } catch {
      continue
    }
    const kotor = bulat(qty.mul(b.harga))
    const dasar = kotor - b.diskon
    let pajakBaris = 0
    let totalBaris = dasar
    if (pajakAktif && aturan!.tax_inclusive) {
      pajakBaris = pajakInklusif(dasar, tarifPajak)
    } else if (pajakAktif) {
      pajakBaris = bulat(new D(dasar).mul(tarifPajak))
      totalBaris = dasar + pajakBaris
    }
    subtotal += kotor
    diskonBaris += b.diskon
    pajak += pajakBaris
    total += totalBaris
  }

  const diskon = diskonBaris + diskonTransaksi
  const dasarLayanan = subtotal - diskon
  const layanan =
    tarifLayanan.greaterThan(0) && dasarLayanan > 0 ? bulat(new D(dasarLayanan).mul(tarifLayanan)) : 0

  return {
    subtotal,
    discount_amount: diskon,
    tax_amount: pajak,
    service_amount: layanan,
    total: total - diskonTransaksi + layanan,
  }
}
