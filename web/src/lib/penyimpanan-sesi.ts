/**
 * Penyimpanan sesi per realm.
 *
 * Backend punya TIGA realm terpisah — tenant, mitra, dan panel internal — dan
 * token satu realm ditolak di realm lain. Maka tokennya pun disimpan di kunci
 * yang berbeda: mitra yang membuka portal tidak boleh "kebetulan" masuk dengan
 * sesi toko yang masih tersimpan di peramban yang sama.
 */
import type { Realm } from './api-client'

export interface Sesi {
  access_token: string
  refresh_token: string
  /** Waktu kedaluwarsa access token (epoch ms) — untuk menjadwalkan pembaruan. */
  kedaluwarsa: number
}

const KUNCI: Record<Realm, string> = {
  tenant: 'pos.sesi.toko',
  mitra: 'pos.sesi.mitra',
  platform: 'pos.sesi.panel',
}

/**
 * Access token ikut disimpan bersama refresh token, LENGKAP dengan waktu
 * kedaluwarsanya.
 *
 * Rancangan awal menahan access token di memori saja demi keamanan. Setelah
 * dijalankan sungguhan, dua hal terlihat:
 *
 *   1. Manfaat keamanannya nyaris nol. Refresh token — kredensial yang JAUH
 *      lebih berkuasa, karena bisa mencetak access token baru berkali-kali —
 *      memang harus menetap di localStorage supaya kasir tidak diminta masuk
 *      ulang tiap pagi. Penyerang yang bisa membaca localStorage sudah
 *      mendapatkan yang lebih besar; menyembunyikan yang kecil tidak menutup
 *      apa pun.
 *
 *   2. Ongkosnya nyata. Memori hilang setiap halaman dimuat ulang, jadi SETIAP
 *      muat ulang memaksa satu panggilan /auth/refresh. Endpoint itu dibatasi
 *      ~0.2 permintaan/detik dengan burst 5 PER ALAMAT IP. Satu warung dengan
 *      tablet kasir, HP pemilik, dan HP gudang berbagi satu IP — mereka saling
 *      menghabiskan jatah dan akhirnya terlempar ke layar masuk bersamaan.
 *
 * Karena itu token disimpan, dan pembaruan hanya dilakukan saat benar-benar
 * kedaluwarsa (atau saat server membalas 401).
 */

const pendengar = new Set<() => void>()

function beritahu() {
  for (const f of pendengar) f()
}

export function langganSesi(f: () => void): () => void {
  pendengar.add(f)
  return () => pendengar.delete(f)
}

export function ambilSesi(realm: Realm): Sesi | null {
  const mentah = localStorage.getItem(KUNCI[realm])
  if (!mentah) return null
  try {
    const sesi = JSON.parse(mentah) as Sesi
    // Token yang sudah lewat waktunya dianggap tidak ada, supaya api-client
    // memperbaruinya lebih dulu alih-alih mengirim permintaan yang pasti 401.
    if (sesi.kedaluwarsa && sesi.kedaluwarsa <= Date.now()) {
      return { ...sesi, access_token: '' }
    }
    return sesi
  } catch {
    localStorage.removeItem(KUNCI[realm])
    return null
  }
}

export function simpanSesi(realm: Realm, sesi: Sesi): void {
  localStorage.setItem(KUNCI[realm], JSON.stringify(sesi))
  beritahu()
}

export function hapusSesi(realm: Realm): void {
  localStorage.removeItem(KUNCI[realm])
  beritahu()
}

export function adaSesi(realm: Realm): boolean {
  return localStorage.getItem(KUNCI[realm]) !== null
}
