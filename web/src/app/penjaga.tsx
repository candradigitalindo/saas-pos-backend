import { Navigate, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { ShieldOff } from 'lucide-react'
import type { KodeIzin } from '@/lib/izin'
import { tujuanSetelahMasuk } from '@/bersama/util/tujuan-masuk'

/** Rute yang wajib masuk dulu. */
export function ButuhMasuk({ children }: { children: ReactNode }) {
  const { sudahMasuk, memuat } = useSesi()
  const lokasi = useLocation()

  if (memuat) return <MemuatHalaman />
  if (!sudahMasuk) return <Navigate to="/masuk" replace state={{ dari: lokasi.pathname }} />
  return <>{children}</>
}

/** Rute publik yang tidak boleh dibuka lagi setelah masuk. */
export function TamuSaja({ children }: { children: ReactNode }) {
  const { sudahMasuk, memuat } = useSesi()
  const lokasi = useLocation()
  if (memuat) return <MemuatHalaman />
  if (sudahMasuk) return <Navigate to={tujuanSetelahMasuk(lokasi.state)} replace />
  return <>{children}</>
}

/**
 * Penjagaan izin di tingkat rute, supaya URL yang diketik langsung tetap
 * tertolak. Ini hanya kenyamanan — PENEGAKAN SESUNGGUHNYA TETAP DI BACKEND.
 */
export function ButuhIzin({
  izin,
  children,
}: {
  izin: KodeIzin[]
  children: ReactNode
}) {
  const { boleh, memuat } = useSesi()
  if (memuat) return <MemuatHalaman />
  if (!boleh(...izin)) {
    return (
      <KeadaanKosong
        ikon={ShieldOff}
        judul="Fitur ini tidak tersedia untuk akun Anda"
        penjelasan="Minta pemilik usaha untuk memberi Anda akses bila memang dibutuhkan."
      />
    )
  }
  return <>{children}</>
}

function MemuatHalaman() {
  return (
    <div className="flex flex-col gap-3 p-4">
      <KerangkaKartuAngka />
      <KerangkaKartuAngka />
    </div>
  )
}
