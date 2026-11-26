// Package money formats normalized transaction amounts.
package money

import (
	"strconv"
	"strings"
)

// Decimal returns the exact signed amount in major units at the recorded scale.
func Decimal(minorUnits int64, scale int) string {
	value := strconv.FormatInt(minorUnits, 10)
	if scale == 0 {
		return value
	}
	digits, negative := strings.CutPrefix(value, "-")
	digits = strings.Repeat("0", max(0, scale+1-len(digits))) + digits
	cut := len(digits) - scale
	amount := digits[:cut] + "." + digits[cut:]
	if negative {
		return "-" + amount
	}
	return amount
}
