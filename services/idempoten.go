package services

import (
	"context"
	"fmt"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// jalankanIdempoten menjalankan satu operasi pencipta uang/stok secara
// idempoten (§8 "Idempotensi"), SELURUHNYA di dalam satu transaksi bertenant:
//
//  1. Cari (scope, key). Ada dan request_hash sama → kembalikan balasan
//     tersimpan tanpa mengerjakan ulang (replayed = true).
//  2. Ada tapi hash berbeda → ErrConflict (409): satu kunci tidak boleh
//     dipakai untuk dua permintaan berbeda.
//  3. Tidak ada → jalankan `kerja`, simpan balasannya di transaksi yang SAMA,
//     lalu kembalikan. Karena satu transaksi, operasi dan catatan
//     idempotensinya jadi atau batal bersama — tidak ada keadaan "uang sudah
//     tercatat tapi kuncinya belum", yang membuat kiriman ulang mencatat dua
//     kali.
//
// Kunci kosong ditolak (ErrValidation): pemanggil yang lupa mengirimnya
// justru yang paling butuh perlindungan ini (tombol ditekan dua kali, jaringan
// putus setelah server sempat menyimpan).
func jalankanIdempoten(
	ctx context.Context,
	scope, key, requestHash string,
	kerja func(tx *gorm.DB) (int, []byte, error),
) (status int, body []byte, replayed bool, err error) {
	if key == "" {
		return 0, nil, false, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, scope, key, requestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			status, body, replayed = m.ResponseStatus, m.ResponseBody, true
			return nil
		}
		st, b, err := kerja(tx)
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, scope, key, requestHash, st, b, idempotencyTTL()); err != nil {
			return err
		}
		status, body = st, b
		return nil
	})
	if err != nil {
		return 0, nil, false, err
	}
	return status, body, replayed, nil
}
