package pkg

import "strconv"

func IntSliceToStringSlice(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}
