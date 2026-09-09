import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Gabung kelas Tailwind; kelas yang bentrok dimenangkan yang terakhir. */
export function cn(...kelas: ClassValue[]): string {
  return twMerge(clsx(kelas))
}
