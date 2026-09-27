import '@testing-library/jest-dom/vitest'

// jsdom tidak punya API tata letak yang dipakai Radix Popover & cmdk (kolom
// Pilihan). Pengganti minimal — tes tidak mengukur posisi, hanya perilaku.
class ResizeObserverTiruan {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= ResizeObserverTiruan as unknown as typeof ResizeObserver
Element.prototype.scrollIntoView ??= function () {}
Element.prototype.hasPointerCapture ??= () => false
Element.prototype.releasePointerCapture ??= function () {}
