package utils

import "math/big"

// Base62Encode qisqa token yaratish uchun

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Base62 kodlash
func Base62Encode(input string) string {
	num := new(big.Int)
	num.SetBytes([]byte(input)) // Convert string to big.Int
	var result []byte
	base := big.NewInt(62)
	for num.Cmp(big.NewInt(0)) > 0 {
		mod := new(big.Int)
		num.DivMod(num, base, mod)
		result = append(result, base62Chars[mod.Int64()])
	}
	// Reverse the result to get the correct base62 encoding
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}
	return string(result)
}
