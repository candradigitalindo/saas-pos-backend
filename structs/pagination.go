package structs

type PaginationLink struct {
	URL    *string `json:"url"`
	Label  string  `json:"label"`
	Active bool    `json:"active"`
}

type PaginatedResponse[T any] struct {
	CurrentPage  int              `json:"current_page"`   // Nomor halaman saat ini.
	Data         []T              `json:"data"`           // Kumpulan data untuk halaman saat ini.
	FirstPageURL string           `json:"first_page_url"` // URL untuk halaman pertama.
	From         int              `json:"from"`           // Nomor urut data pertama di halaman ini.
	LastPage     int              `json:"last_page"`      // Nomor halaman terakhir.
	LastPageURL  string           `json:"last_page_url"`  // URL untuk halaman terakhir.
	Links        []PaginationLink `json:"links"`          // Daftar tautan paginasi (termasuk "next", "prev", dan nomor halaman).
	NextPageURL  *string          `json:"next_page_url"`  // URL untuk halaman berikutnya, atau null jika ini halaman terakhir.
	Path         string           `json:"path"`           // Path dasar untuk resource yang dipaginasi.
	PerPage      int              `json:"per_page"`       // Jumlah item yang ditampilkan per halaman.
	PrevPageURL  *string          `json:"prev_page_url"`  // URL untuk halaman sebelumnya, atau null jika ini halaman pertama.
	To           int              `json:"to"`             // Nomor urut data terakhir di halaman ini.
	Total        int64            `json:"total"`          // Jumlah total semua item yang tersedia.
}
