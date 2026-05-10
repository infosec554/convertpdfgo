package models

type UserStats struct {
	Merged           int `json:"merged"`             // Birlashtirish ishlar soni
	Splitted         int `json:"splitted"`           // Bo‘lib olish ishlar soni
	RemovedPages     int `json:"removed_pages"`      // Sahifa o‘chirish ishlar soni
	Compressed       int `json:"compressed"`         // Siqilgan fayllar soni
	Extracted        int `json:"extracted"`          // Sahifalar ajratilgan ishlar
	Organized        int `json:"organized"`          // Sahifa tartibi o‘zgartirilgan
	JPGToPDF         int `json:"jpg_to_pdf"`         // JPG → PDF aylantirishlar
	PDFToJPG         int `json:"pdf_to_jpg"`         // PDF → JPG aylantirishlar
	PDFToWord        int `json:"pdf_to_word"`        // PDF → Word aylantirishlar
	Rotated          int `json:"rotated"`            // Sahifa aylantirishlar
	Cropped          int `json:"cropped"`            // PDFni kesish ishlari
	AddedPageNumbers int `json:"added_page_numbers"` // Sahifalarga raqam qo‘shish
	Unlocked         int `json:"unlocked"`           // PDF qulflar yechilgan soni
	Protected        int `json:"protected"`          // Parol bilan himoyalanganlar
	Watermarked      int `json:"watermarked"`        // Suv belgisi qo‘shilganlar

	TotalFiles    int `json:"total_files"`     // Umumiy yuklangan fayllar
	UsedStorageMB int `json:"used_storage_mb"` // MB’da ishlatilgan xotira (hisoblash kerak)
}
