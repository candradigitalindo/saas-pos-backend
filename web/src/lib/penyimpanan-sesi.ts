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
 * Access token ditahan di memori supaya tidak ikut terbaca skrip pihak ketiga;
 * refresh token menetap di localStorage supaya kasir tidak diminta masuk ulang
 * setiap membuka aplikasi.
 */
const diMemori = new Map<Realm, string>()

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
    const tersimpan = JSON.parse(mentah) as Omit<Sesi, 'access_token'>
    return {
      ...tersimpan,
      access_token: diMemori.get(realm) ?? '',
    }
  } catch {
    localStorage.removeItem(KUNCI[realm])
    return null
  }
}

export function simpanSesi(realm: Realm, sesi: Sesi): void {
  diMemori.set(realm, sesi.access_token)
  localStorage.setItem(
    KUNCI[realm],
    JSON.stringify({ refresh_token: sesi.refresh_token, kedaluwarsa: sesi.kedaluwarsa }),
  )
  beritahu()
}

export function hapusSesi(realm: Realm): void {
  diMemori.delete(realm)
  localStorage.removeItem(KUNCI[realm])
  beritahu()
}

export function adaSesi(realm: Realm): boolean {
  return localStorage.getItem(KUNCI[realm]) !== null
}
