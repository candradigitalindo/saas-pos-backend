import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, ExternalLink, PlugZap, TriangleAlert, Unplug } from 'lucide-react'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { kanalApi, type Kanal, type PenyediaKanal, type SambunganKanal } from '../api'

/** Nama bebas saat membuat kanal → kode penyedia (sama dengan services.ProviderCode). */
const ALIAS: Record<string, string> = {
  whatsapp: 'whatsapp', wa: 'whatsapp', 'whatsapp business': 'whatsapp',
  gofood: 'gofood', gobiz: 'gofood', grabfood: 'grabfood', grab: 'grabfood', shopee: 'shopee',
  'tiktok shop': 'tiktokshop', tiktokshop: 'tiktokshop', tokopedia: 'tiktokshop', lazada: 'lazada',
}
export const kodePenyedia = (k: Kanal) =>
  ALIAS[k.provider.trim().toLowerCase()] ?? ALIAS[k.name.trim().toLowerCase()]

/**
 * Menyambungkan kanal ke API penyedia dengan KREDENSIAL MILIK TOKO sendiri.
 *
 * Platform tidak mengurus izin: toko mendaftar sendiri di Meta/GoBiz/Shopee,
 * lalu menempelkan kredensialnya di sini. Layar ini menjawab tiga hal secara
 * berurutan — dari mana kredensialnya, apakah sudah benar (tes koneksi dengan
 * pesan asli penyedia), dan apa yang harus ditempel balik di konsol penyedia
 * (alamat webhook). Rahasia tidak pernah dikirim balik; yang tampil hanya
 * empat karakter terakhir.
 */
export function DialogSambungan({ kanal, onTutup }: { kanal: Kanal; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const penyedia = useQuery({ queryKey: ['penyedia-kanal'], queryFn: kanalApi.penyedia, staleTime: 5 * 60_000 })
  const sambungan = useQuery({
    queryKey: ['sambungan-kanal', kanal.id],
    queryFn: () => kanalApi.sambungan(kanal.id),
  })
  const [kodePilihan, setKode] = useState<string | undefined>(undefined)
  const [isian, setIsian] = useState<Record<string, string>>({})
  const [hasil, setHasil] = useState<SambunganKanal | null>(null)
  const [galat, setGalat] = useState<string | null>(null)
  const [yakinPutus, setYakinPutus] = useState(false)

  const s = hasil ?? sambungan.data
  // Sebelum dipilih: penyedia yang tersambung, tebakan dari nama kanal, lalu
  // yang pertama tersedia.
  const tebak = sambungan.data?.provider ?? kodePenyedia(kanal)
  const kode =
    kodePilihan ??
    penyedia.data?.find((p) => p.code === tebak)?.code ??
    penyedia.data?.find((p) => p.available)?.code
  const dipilih = useMemo(() => penyedia.data?.find((p) => p.code === kode), [penyedia.data, kode])
  const tersimpan = !!s?.provider && s.provider === kode

  const simpan = useMutation({
    mutationFn: async () => {
      await kanalApi.simpanSambungan(kanal.id, kode!, isian)
      return kanalApi.tesSambungan(kanal.id)
    },
    onSuccess: (r) => {
      setHasil(r)
      setIsian({})
      qc.invalidateQueries({ queryKey: ['kanal'] })
      qc.invalidateQueries({ queryKey: ['sambungan-kanal', kanal.id] })
      if (r.status === 'connected') toast.berhasil(`${kanal.name} tersambung.`)
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal menyimpan.'),
  })
  const putus = useMutation({
    mutationFn: () => kanalApi.putusSambungan(kanal.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['kanal'] })
      qc.invalidateQueries({ queryKey: ['sambungan-kanal', kanal.id] })
      toast.berhasil(`Sambungan API ${kanal.name} diputus. Kanal kembali manual.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal memutus.'),
  })

  const kurang = dipilih?.fields.some(
    (f) => !f.optional && !(isian[f.key] ?? '').trim() && !(tersimpan && s?.fields[f.key]?.set),
  )

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog
        judul={`Sambungan API ${kanal.name}`}
        keterangan="Pakai akun developer milik toko Anda sendiri — pesanan lalu masuk otomatis."
        className="sm:max-w-xl"
      >
        {penyedia.isLoading || sambungan.isLoading ? (
          <KerangkaBaris jumlah={3} />
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label="Penyedia">
              {(penyedia.data ?? []).map((p) => (
                <button
                  key={p.code}
                  type="button"
                  role="radio"
                  aria-checked={kode === p.code}
                  onClick={() => {
                    setKode(p.code)
                    setIsian({})
                    setGalat(null)
                  }}
                  className={cn(
                    'flex min-h-11 items-center gap-1.5 rounded-full border px-3 text-label',
                    kode === p.code
                      ? 'border-utama bg-sorot font-semibold text-utama'
                      : 'border-garis text-teks-sekunder hover:text-teks-utama',
                  )}
                >
                  {p.name}
                  {!p.available && <span className="text-keterangan font-normal text-teks-redup">· segera</span>}
                </button>
              ))}
            </div>

            {dipilih && !dipilih.available && <BelumTersedia p={dipilih} />}

            {dipilih?.available && (
              <>
                {tersimpan && s && <StatusSambungan s={s} />}

                <ul className="flex flex-col gap-1.5 rounded-kontrol bg-permukaan-2 px-3 py-2.5 text-label">
                  {dipilih.capabilities.map((c) => (
                    <li key={c} className="flex items-start gap-2 text-teks-sekunder">
                      <Check className="mt-0.5 h-4 w-4 shrink-0 text-hijau-800" aria-hidden />
                      {c}
                    </li>
                  ))}
                </ul>

                <details className="group rounded-kontrol border border-garis" open={!tersimpan}>
                  <summary className="flex min-h-11 cursor-pointer items-center justify-between gap-2 px-3 text-label font-semibold text-teks-utama">
                    Cara mendapatkan kredensialnya
                    <a
                      href={dipilih.docs_url}
                      target="_blank"
                      rel="noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="inline-flex items-center gap-1 font-medium text-utama hover:underline"
                    >
                      Dokumentasi
                      <ExternalLink className="h-3.5 w-3.5" aria-hidden />
                    </a>
                  </summary>
                  <ol className="flex list-decimal flex-col gap-1.5 px-3 pb-3 pl-8 text-label text-teks-sekunder">
                    {dipilih.steps.map((l) => (
                      <li key={l}>{l}</li>
                    ))}
                  </ol>
                </details>

                <div className="flex flex-col gap-3">
                  {dipilih.fields.map((f) => {
                    const lama = tersimpan ? s?.fields[f.key] : undefined
                    return (
                      <Kolom
                        key={f.key}
                        label={f.label}
                        type={f.secret ? 'password' : 'text'}
                        autoComplete="off"
                        value={isian[f.key] ?? (f.secret ? '' : (lama?.value ?? ''))}
                        onChange={(e) => setIsian((x) => ({ ...x, [f.key]: e.target.value }))}
                        placeholder={
                          f.secret && lama?.set ? `Tersimpan (${lama.preview}) — kosongkan bila tidak diganti` : undefined
                        }
                        bantuan={f.help}
                        required={!f.optional}
                      />
                    )
                  })}
                </div>

                {tersimpan && s?.webhook_url && <AlamatWebhook s={s} />}

                {galat && (
                  <p role="alert" className="text-label text-bahaya-teks">
                    {galat}
                  </p>
                )}

                {yakinPutus ? (
                  <div className="flex flex-col gap-2 rounded-kontrol border border-garis p-3">
                    <p className="text-label text-teks-sekunder">
                      Kredensial dihapus dan alamat webhook berhenti menerima pesanan. Pesanan yang sudah
                      tercatat tetap ada; kanal kembali dicatat manual.
                    </p>
                    <div className="flex flex-wrap gap-2">
                      <Tombol jenis="bahaya" ukuran="padat" onClick={() => putus.mutate()} memuat={putus.isPending}>
                        Ya, putuskan
                      </Tombol>
                      <Tombol jenis="kedua" ukuran="padat" onClick={() => setYakinPutus(false)}>
                        Batal
                      </Tombol>
                    </div>
                  </div>
                ) : null}

                <AksiDialog>
                  <Tombol
                    onClick={() => {
                      setGalat(null)
                      simpan.mutate()
                    }}
                    disabled={kurang}
                    memuat={simpan.isPending}
                    labelMemuat="Menyimpan & mengetes…"
                  >
                    <PlugZap className="h-5 w-5" aria-hidden />
                    {tersimpan ? 'Simpan & Tes Ulang' : 'Simpan & Tes Koneksi'}
                  </Tombol>
                  {tersimpan && !yakinPutus && (
                    <Tombol jenis="kedua" onClick={() => setYakinPutus(true)}>
                      <Unplug className="h-5 w-5" aria-hidden />
                      Putuskan
                    </Tombol>
                  )}
                </AksiDialog>
              </>
            )}
          </div>
        )}
      </IsiDialog>
    </Dialog>
  )
}

function StatusSambungan({ s }: { s: SambunganKanal }) {
  if (s.status === 'connected') {
    return (
      <div className="flex items-start gap-2 rounded-kontrol bg-hijau-700/10 px-3 py-2.5 text-label text-hijau-800">
        <Check className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
        <div>
          <p className="font-semibold">Tersambung{s.info ? ` — ${s.info}` : ''}</p>
          <p className="text-keterangan">
            {s.last_event_at
              ? `Pesanan otomatis terakhir ${formatTanggalJam(s.last_event_at)}`
              : 'Belum ada pesanan otomatis masuk. Pastikan alamat webhook di bawah sudah ditempel.'}
          </p>
        </div>
      </div>
    )
  }
  if (s.status === 'error') {
    return (
      <div className="flex items-start gap-2 rounded-kontrol border border-jingga-600/60 bg-permukaan-2 px-3 py-2.5 text-label">
        <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
        <div>
          <p className="font-semibold text-jingga-700">Tes koneksi gagal</p>
          <p className="text-teks-sekunder">{s.error}</p>
        </div>
      </div>
    )
  }
  return (
    <p className="rounded-kontrol bg-permukaan-2 px-3 py-2.5 text-label text-teks-sekunder">
      Kredensial tersimpan, belum dites.
    </p>
  )
}

/** Yang harus ditempel balik di konsol penyedia. */
function AlamatWebhook({ s }: { s: SambunganKanal }) {
  const relatif = s.webhook_url?.startsWith('/')
  const baris = [{ label: 'Callback URL', value: s.webhook_url! }, ...(s.webhook_values ?? [])]
  return (
    <div className="flex flex-col gap-2 rounded-kontrol border border-garis p-3">
      <p className="text-label font-semibold text-teks-utama">Tempel di konsol penyedia</p>
      {baris.map((b) => (
        <SalinNilai key={b.label} label={b.label} nilai={b.value} />
      ))}
      {relatif && (
        <p className="flex items-start gap-1.5 text-keterangan text-jingga-700">
          <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
          Alamat publik server belum diatur (APP_URL), jadi penyedia belum bisa menjangkau alamat ini.
        </p>
      )}
    </div>
  )
}

function SalinNilai({ label, nilai }: { label: string; nilai: string }) {
  const toast = useToast()
  return (
    <div className="flex flex-col gap-1">
      <span className="text-keterangan text-teks-redup">{label}</span>
      <div className="flex items-center gap-2">
        <code className="min-w-0 flex-1 truncate rounded-kontrol bg-permukaan-2 px-2.5 py-2 text-keterangan text-teks-utama">
          {nilai}
        </code>
        <button
          type="button"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(nilai)
              toast.berhasil(`${label} disalin.`)
            } catch {
              toast.gagal('Tidak bisa menyalin — pilih teksnya lalu salin manual.')
            }
          }}
          aria-label={`Salin ${label}`}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol border border-garis text-teks-sekunder hover:text-teks-utama"
        >
          <Copy className="h-4 w-4" aria-hidden />
        </button>
      </div>
    </div>
  )
}

function BelumTersedia({ p }: { p: PenyediaKanal }) {
  return (
    <div className="flex flex-col gap-2 rounded-kontrol border border-dashed border-garis p-4 text-label">
      <p className="font-semibold text-teks-utama">Sambungan {p.name} sedang disiapkan</p>
      {p.note && <p className="text-teks-sekunder">{p.note}</p>}
      <p className="text-teks-sekunder">
        Sementara itu, catat pesanan manual atau impor laporan harian (CSV) — angka dan stoknya tetap
        masuk ke tempat yang sama.
      </p>
      <a
        href={p.docs_url}
        target="_blank"
        rel="noreferrer"
        className="inline-flex w-fit items-center gap-1 font-medium text-utama hover:underline"
      >
        Portal developer {p.name}
        <ExternalLink className="h-3.5 w-3.5" aria-hidden />
      </a>
    </div>
  )
}
