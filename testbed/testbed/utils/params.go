package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseIntArray(value string) ([]uint64, error) {
	var ints []uint64
	strs := strings.Split(value, ",")
	for _, str := range strs {
		num, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Could not convert '%s' to integer(s)", strs)
		}
		ints = append(ints, num)
	}
	return ints, nil
}
