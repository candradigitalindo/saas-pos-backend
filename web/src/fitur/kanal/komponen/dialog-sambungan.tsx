import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, ExternalLink, Loader2, PlugZap, Store, TriangleAlert, Unplug } from 'lucide-react'
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
  // Tokopedia & TikTok Shop di Indonesia = satu toko, satu API ("Tokopedia & Shop").
  tokopedia: 'tokopedia', 'tokopedia & shop': 'tokopedia', 'tiktok shop': 'tokopedia', tiktokshop: 'tokopedia',
  tiktok: 'tokopedia', lazada: 'lazada',
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
 *
 * Penyedia ber-OAuth (Shopee, Tokopedia & Shop) menambah satu langkah:
 * setelah kredensial aplikasi tersimpan, pemilik toko memberi izin di tab
 * baru. Dialog ini menunggu sendiri (memeriksa sambungan tiap 3 detik) sampai
 * izinnya tercatat.
 */
export function DialogSambungan({ kanal, onTutup }: { kanal: Kanal; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const penyedia = useQuery({ queryKey: ['penyedia-kanal'], queryFn: kanalApi.penyedia, staleTime: 5 * 60_000 })
  // Menunggu izin toko: `sejak` = checked_at saat tab izin dibuka. Callback
  // yang berhasil memperbaruinya — begitu pula saat otorisasi ULANG, ketika
  // nama toko sudah terisi sejak awal.
  const [menunggu, setMenunggu] = useState<{ sejak: string } | null>(null)
  const [tautanIzin, setTautanIzin] = useState<string | null>(null)
  const sambungan = useQuery({
    queryKey: ['sambungan-kanal', kanal.id],
    queryFn: () => kanalApi.sambungan(kanal.id),
    refetchInterval: () => (menunggu ? 3000 : false),
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
  const perluIzin = !!dipilih?.requires_authorization
  const belumIzin = perluIzin && !s?.authorized

  const diizinkan = sambungan.data?.authorized
  const dicek = sambungan.data?.checked_at ?? ''
  // Setelah izin tercatat, tes koneksi langsung dijalankan: hasilnya (toko,
  // wilayah, webhook terdaftar atau belum) tampil tanpa perlu menekan apa pun.
  const tesIzin = useMutation({
    mutationFn: () => kanalApi.tesSambungan(kanal.id),
    onSuccess: (r) => {
      setHasil(r)
      qc.invalidateQueries({ queryKey: ['kanal'] })
    },
  })
  useEffect(() => {
    if (!menunggu || !diizinkan || dicek === menunggu.sejak) return
    setMenunggu(null)
    setTautanIzin(null)
    setHasil(null)
    qc.invalidateQueries({ queryKey: ['kanal'] })
    tesIzin.mutate()
    toast.berhasil(`${diizinkan} sudah memberi izin — pesanan baru akan masuk otomatis.`)
  }, [menunggu, diizinkan, dicek, qc, toast, tesIzin])

  const izin = useMutation({ mutationFn: () => kanalApi.otorisasi(kanal.id) })
  const mulaiIzin = () => {
    setGalat(null)
    setTautanIzin(null)
    // Tab dibuka SEBELUM menunggu server: peramban hanya mengizinkan jendela
    // baru langsung dari klik. Bila tetap diblokir, tautannya ditampilkan.
    const tab = window.open('', '_blank')
    izin.mutate(undefined, {
      onSuccess: ({ url }) => {
        if (tab && !tab.closed) {
          tab.opener = null
          tab.location.href = url
        } else {
          setTautanIzin(url)
        }
        setHasil(null)
        setMenunggu({ sejak: sambungan.data?.checked_at ?? '' })
      },
      onError: (e) => {
        tab?.close()
        setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal membuka halaman izin.')
      },
    })
  }

  const simpan = useMutation({
    mutationFn: async () => {
      const r = await kanalApi.simpanSambungan(kanal.id, kode!, isian)
      // Belum ada izin toko → tes pasti gagal; jangan catat sebagai galat.
      if (r.needs_authorization && !r.authorized) return r
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
  // Izin memakai kredensial TERSIMPAN — isian yang belum disimpan harus disimpan dulu.
  const adaUbahan = Object.entries(isian).some(
    ([k, v]) => v.trim() !== '' && v !== (tersimpan ? s?.fields[k]?.value : undefined),
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
                {tersimpan && s && !belumIzin && <StatusSambungan s={s} />}

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
                    const pilihan = f.options ?? []
                    if (pilihan.length > 0) {
                      const nilai = isian[f.key] ?? lama?.value ?? pilihan[0]!.value
                      return (
                        <div key={f.key} className="flex flex-col gap-1.5">
                          <p id={`label-${f.key}`} className="text-label font-medium text-teks-sekunder">
                            {f.label}
                          </p>
                          <div
                            role="radiogroup"
                            aria-labelledby={`label-${f.key}`}
                            className="grid auto-cols-fr grid-flow-col gap-1 rounded-full bg-permukaan-2 p-1"
                          >
                            {pilihan.map((o) => (
                              <button
                                key={o.value}
                                type="button"
                                role="radio"
                                aria-checked={nilai === o.value}
                                onClick={() => setIsian((x) => ({ ...x, [f.key]: o.value }))}
                                className={cn(
                                  'h-11 rounded-full px-2 text-label font-semibold',
                                  nilai === o.value
                                    ? 'bg-permukaan text-utama shadow-kartu'
                                    : 'text-teks-sekunder hover:text-teks-utama',
                                )}
                              >
                                {o.label}
                              </button>
                            ))}
                          </div>
                          {f.help && <p className="text-keterangan text-teks-redup">{f.help}</p>}
                        </div>
                      )
                    }
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

                {tersimpan && s?.webhook_url && (
                  <AlamatWebhook s={s} label={dipilih.webhook_label ?? 'Callback URL'} />
                )}

                {tersimpan && perluIzin && (
                  <IzinToko
                    penyedia={dipilih.name}
                    redirect={s?.webhook_values?.find((v) => v.label.startsWith('Redirect'))?.label ?? 'Redirect URL'}
                    alasan={belumIzin && s?.status === 'error' ? s.error : undefined}
                    toko={s?.authorized}
                    menunggu={!!menunggu}
                    tautan={tautanIzin}
                    memuat={izin.isPending}
                    terkunci={adaUbahan}
                    onMulai={mulaiIzin}
                  />
                )}

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
                    // Saat menunggu izin toko, tombol utamanya ada di kotak izin.
                    jenis={tersimpan && belumIzin && !adaUbahan ? 'kedua' : 'utama'}
                    onClick={() => {
                      setGalat(null)
                      simpan.mutate()
                    }}
                    disabled={kurang}
                    memuat={simpan.isPending}
                    labelMemuat={belumIzin ? 'Menyimpan…' : 'Menyimpan & mengetes…'}
                  >
                    <PlugZap className="h-5 w-5" aria-hidden />
                    {belumIzin
                      ? tersimpan
                        ? 'Simpan Perubahan'
                        : 'Simpan Kredensial'
                      : tersimpan
                        ? 'Simpan & Tes Ulang'
                        : 'Simpan & Tes Koneksi'}
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

/**
 * Langkah izin toko (OAuth). Belum diizinkan: ajakan utama dengan pengingat
 * isian redirect di konsol penyedia — penyebab gagal paling umum. Sudah: nama tokonya, plus
 * jalan otorisasi ulang (ganti toko / izin dicabut / kedaluwarsa 1 tahun).
 */
function IzinToko({
  penyedia,
  redirect,
  alasan,
  toko,
  menunggu,
  tautan,
  memuat,
  terkunci,
  onMulai,
}: {
  penyedia: string
  /** Nama isian redirect di konsol penyedia ("Redirect URL Domain" di Shopee). */
  redirect: string
  /** Izin sebelumnya hilang (mis. toko mencabutnya di Seller Center). */
  alasan?: string
  toko?: string
  menunggu: boolean
  tautan: string | null
  memuat: boolean
  terkunci: boolean
  onMulai: () => void
}) {
  const tunggu = (
    <>
      {menunggu && (
        <p role="status" className="flex items-center gap-2 text-label text-teks-sekunder">
          <Loader2 className="h-4 w-4 shrink-0 animate-spin motion-reduce:animate-none" aria-hidden />
          Menunggu izin dari {penyedia}… layar ini diperbarui sendiri.
        </p>
      )}
      {tautan && (
        <a
          href={tautan}
          target="_blank"
          rel="noreferrer"
          className="inline-flex w-fit items-center gap-1 text-label font-medium text-utama hover:underline"
        >
          Peramban memblokir tab baru — buka halaman izin {penyedia}
          <ExternalLink className="h-3.5 w-3.5" aria-hidden />
        </a>
      )}
      {terkunci && (
        <p className="text-keterangan text-teks-redup">Simpan dulu perubahan kredensial sebelum memberi izin.</p>
      )}
    </>
  )
  if (toko) {
    return (
      <div className="flex flex-col gap-2 rounded-kontrol border border-garis p-3">
        {/* Layar sempit: tombol turun ke baris sendiri — nama toko Shopee
            sering panjang ("… Official Store") dan tidak boleh terpotong. */}
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex min-w-0 flex-1 basis-56 items-center gap-3">
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sorot text-utama">
              <Store className="h-5 w-5" aria-hidden />
            </span>
            <div className="min-w-0">
              <p className="text-keterangan text-teks-redup">Toko yang memberi izin</p>
              <p className="text-label font-semibold break-words text-teks-utama">{toko}</p>
            </div>
          </div>
          <Tombol
            jenis="kedua"
            ukuran="padat"
            className="w-full sm:w-auto"
            onClick={onMulai}
            disabled={terkunci}
            memuat={memuat}
          >
            Otorisasi Ulang
          </Tombol>
        </div>
        {tunggu}
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-3 rounded-kontrol border border-utama/40 bg-sorot p-3">
      <div className="flex items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-permukaan text-utama">
          <Store className="h-5 w-5" aria-hidden />
        </span>
        <div className="flex min-w-0 flex-col gap-0.5 text-label">
          <p className="font-semibold text-teks-utama">
            {alasan ? `Izin toko ${penyedia} perlu diberikan ulang` : `Langkah terakhir: izin toko ${penyedia}`}
          </p>
          {alasan && (
            <p className="flex items-start gap-1.5 text-jingga-700">
              <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              {alasan}
            </p>
          )}
          <p className="text-teks-sekunder">
            Pastikan {redirect} di atas sudah diisi di aplikasi {penyedia}. Halaman izin terbuka di tab
            baru — masuk dengan akun penjual, lalu setujui.
          </p>
        </div>
      </div>
      {tunggu}
      <Tombol
        jenis={menunggu ? 'kedua' : 'utama'}
        lebarPenuh
        onClick={onMulai}
        disabled={terkunci}
        memuat={memuat}
        labelMemuat="Membuka…"
      >
        <ExternalLink className="h-5 w-5" aria-hidden />
        {menunggu ? 'Buka Lagi Halaman Izin' : `Otorisasi Toko ${penyedia}`}
      </Tombol>
    </div>
  )
}

/** Yang harus ditempel balik di konsol penyedia. */
function AlamatWebhook({ s, label }: { s: SambunganKanal; label: string }) {
  const relatif = s.webhook_url?.startsWith('/')
  const baris = [{ label, value: s.webhook_url! }, ...(s.webhook_values ?? [])]
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
        {/* Yang terpotong bagian DEPAN (elipsis), 18 karakter terakhir selalu
            tampak: alamat-alamat webhook sama awalnya dan berbeda di ujung
            (".../oauth/token"). Kepalanya inline-block, BUKAN butir flex —
            butir flex membuat blok teks yang disalin manual berisi baris baru
            di tengah alamat. Tombol salin tetap menyalin nilai utuh. */}
        <code
          title={nilai}
          className="block min-w-0 flex-1 overflow-hidden rounded-kontrol bg-permukaan-2 px-2.5 py-2 font-mono text-keterangan whitespace-nowrap text-teks-utama"
        >
          {nilai.length > 24 ? (
            <>
              <span className="inline-block max-w-[calc(100%-18ch)] truncate align-bottom">{nilai.slice(0, -18)}</span>
              {nilai.slice(-18)}
            </>
          ) : (
            nilai
          )}
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
