import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { useToast } from '@/bersama/komponen/toast'
import { useDaftarHarga } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import type { Pelanggan } from '@/bersama/tipe/pos'
import { nomorWA } from '@/fitur/kasir/struk-wa'
import { pelangganApi } from '../api'

/**
 * Tambah / ubah pelanggan. Nomor HP diperiksa sebagai nomor WhatsApp (format
 * apa pun) supaya tombol "Tagih via WA" benar-benar membuka percakapannya.
 * Tempo kasbon memberi kasbon baru dari kasir jatuh tempo otomatis.
 */
export function DialogPelanggan({ awal, onTutup }: { awal: Pelanggan | null; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState(awal?.name ?? '')
  const [hp, setHp] = useState(awal?.phone ?? '')
  const [batas, setBatas] = useState(awal?.credit_limit ?? 0)
  const [tempo, setTempo] = useState(String(awal?.credit_term_days ?? 0))
  const [daftar, setDaftar] = useState(awal?.price_list_id ?? '')
  const [alamat, setAlamat] = useState(awal?.address ?? '')
  const [catatan, setCatatan] = useState(awal?.note ?? '')
  const daftarHarga = useDaftarHarga()
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const nomorSalah = hp.trim() !== '' && nomorWA(hp) === null
  const hariTempo = Number.parseInt(tempo, 10)
  const tempoSalah = tempo.trim() !== '' && (!Number.isInteger(hariTempo) || hariTempo < 0 || hariTempo > 365)

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        phone: hp.trim() || undefined,
        credit_limit: batas,
        credit_term_days: tempo.trim() === '' ? 0 : hariTempo,
        price_list_id: daftar,
        address: alamat.trim(),
        note: catatan.trim(),
      }
      return awal ? pelangganApi.ubah(awal.id, isi) : pelangganApi.buat(isi)
    },
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      qc.invalidateQueries({ queryKey: ['kasbon'] })
      toast.berhasil(awal ? `${p.name} diperbarui.` : `${p.name} ditambahkan.`)
      onTutup()
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else setGalat('Terjadi kesalahan. Coba lagi.')
    },
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog judul={awal ? 'Ubah Pelanggan' : 'Tambah Pelanggan'}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (tempoSalah) return
            setGalat(null)
            setKolomGalat({})
            simpan.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            galat={galatKolom(kolomGalat, 'name')}
            maxLength={150}
            autoFocus
            required
          />
          <Kolom
            label="Nomor WhatsApp"
            type="tel"
            inputMode="tel"
            value={hp}
            onChange={(e) => setHp(e.target.value)}
            placeholder="0812…"
            // Bukan penghalang: nomor lama pelanggan boleh saja bukan WhatsApp.
            bantuan={
              nomorSalah
                ? 'Tidak dikenali sebagai nomor WhatsApp — tagihan tidak bisa dikirim lewat WA.'
                : 'Boleh dikosongkan. Dipakai untuk menagih kasbon lewat WhatsApp.'
            }
            galat={galatKolom(kolomGalat, 'phone')}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <KolomUang
              label="Batas kasbon"
              nilai={batas}
              onNilai={setBatas}
              bantuan="Rp 0 = tidak dibatasi."
              galat={galatKolom(kolomGalat, 'credit_limit')}
            />
            <Kolom
              label="Tempo kasbon"
              inputMode="numeric"
              value={tempo}
              onChange={(e) => setTempo(e.target.value.replace(/\D/g, '').slice(0, 3))}
              akhiran="hari"
              bantuan="0 = tanpa jatuh tempo."
              galat={tempoSalah ? 'Paling lama 365 hari.' : galatKolom(kolomGalat, 'credit_term_days')}
            />
          </div>
          {/* Hanya tampil bila toko sudah membuat daftar harga khusus. */}
          {(daftarHarga.data?.length ?? 0) > 0 && (
            <Pilihan
              label="Daftar harga"
              value={daftar}
              onChange={(e) => setDaftar(e.target.value)}
              bantuan="Di kasir, pelanggan ini mendapat harga khusus daftarnya."
              galat={galatKolom(kolomGalat, 'price_list_id')}
            >
              <option value="">Harga umum</option>
              {daftarHarga.data!.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Pilihan>
          )}
          <Kolom
            label="Alamat"
            value={alamat}
            onChange={(e) => setAlamat(e.target.value)}
            maxLength={255}
            placeholder="Boleh dikosongkan"
            galat={galatKolom(kolomGalat, 'address')}
          />
          <Kolom
            label="Catatan"
            value={catatan}
            onChange={(e) => setCatatan(e.target.value)}
            maxLength={500}
            placeholder="Mis. langganan tiap pagi, suka kopi tanpa gula"
            galat={galatKolom(kolomGalat, 'note')}
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim() || tempoSalah}>
              Simpan
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
