package generateotpcode

import (
	"math/rand"
	"strconv"
)

func GenerateOTPCode(length int) string {
	code := ""
	for i := 0; i < length; i++ {
		code += strconv.Itoa(rand.Intn(10))
	}
	return code
}
