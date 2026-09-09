import { Outlet } from 'react-router-dom'

/** Layout untuk halaman publik: masuk & daftar. Tanpa navigasi apa pun. */
export function LayoutKosong() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center bg-latar px-4 py-8">
      <div className="w-full max-w-md">
        <Outlet />
      </div>
    </div>
  )
}
