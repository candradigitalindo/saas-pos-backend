import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import type { Pemasok } from '@/bersama/tipe/katalog'
import { nomorWA } from '@/fitur/kasir/struk-wa'
import { pemasokApi } from '../api'

/**
 * Tambah / ubah pemasok. Nomor WhatsApp diperiksa saat diketik (format apa
 * pun: "0812-…", "+62 812…") supaya tombol "Pesan lewat WhatsApp" di halaman
 * pemasok benar-benar membuka percakapannya.
 */
export function DialogPemasok({
  terbuka,
  pemasok,
  onTutup,
  onTersimpan,
}: {
  terbuka: boolean
  /** Diisi = ubah; kosong = tambah. */
  pemasok?: Pemasok | null
  onTutup: () => void
  onTersimpan?: (p: Pemasok) => void
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [nama, setNama] = useState('')
  const [telepon, setTelepon] = useState('')
  const [alamat, setAlamat] = useState('')
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  useEffect(() => {
    if (!terbuka) return
    setNama(pemasok?.name ?? '')
    setTelepon(pemasok?.phone ?? '')
    setAlamat(pemasok?.address ?? '')
    setCatatan(pemasok?.note ?? '')
    setGalat(null)
  }, [terbuka, pemasok])

  const nomorSalah = telepon.trim() !== '' && nomorWA(telepon) === null

  const simpan = useMutation({
    mutationFn: () => {
      const input = { name: nama.trim(), phone: telepon.trim(), address: alamat.trim(), note: catatan.trim() }
      return pemasok ? pemasokApi.ubah(pemasok.id, input) : pemasokApi.buat(input)
    },
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['pemasok'] })
      qc.invalidateQueries({ queryKey: ['katalog-pemasok'] })
      toast.berhasil(pemasok ? `${p.name} diperbarui.` : `${p.name} ditambahkan.`)
      onTersimpan?.(p)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul={pemasok ? 'Ubah pemasok' : 'Tambah pemasok'}>
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            if (!nama.trim() || nomorSalah) return
            setGalat(null)
            simpan.mutate()
          }}
        >
          <Kolom label="Nama" value={nama} onChange={(e) => setNama(e.target.value)} maxLength={150} required autoFocus />
          <Kolom
            label="Nomor WhatsApp"
            type="tel"
            inputMode="tel"
            value={telepon}
            onChange={(e) => setTelepon(e.target.value)}
            maxLength={30}
            placeholder="0812…"
            galat={nomorSalah ? 'Nomor belum benar — contoh 0812 3456 7890.' : undefined}
            bantuan={nomorSalah ? undefined : 'Untuk memesan lewat WhatsApp. Boleh dikosongkan.'}
          />
          <Kolom label="Alamat" value={alamat} onChange={(e) => setAlamat(e.target.value)} maxLength={255} />
          <Kolom
            label="Catatan"
            value={catatan}
            onChange={(e) => setCatatan(e.target.value)}
            maxLength={500}
            placeholder="mis. kirim tiap Selasa, minimal order 5 dus"
          />
          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}
          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim() || nomorSalah}>
              Simpan
            </Tombol>
            <Tombol type="button" jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
