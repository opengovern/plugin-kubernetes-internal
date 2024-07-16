package shared

import (
	"fmt"
	"math"
)

func SizeByte(v float64, includeSign bool) string {
	return SizeByte64(v, includeSign)
}

func SizeByte64(v float64, includeSign bool) string {
	format := "%"
	if includeSign {
		format += "+"
	}

	if math.Abs(v) < 1024 {
		format += ".0f Bytes"
		return fmt.Sprintf(format, v)
	}
	v = v / 1024
	if math.Abs(v) < 1024 {
		format += ".1f KB"
		return fmt.Sprintf(format, v)
	}
	v = v / 1024
	if math.Abs(v) < 1024 {
		format += ".1f MB"
		return fmt.Sprintf(format, v)
	}
	v = v / 1024

	format += ".1f GB"
	return fmt.Sprintf(format, v)
}

func SizeByte64WithStyle(value float64, includeSign bool) string {
	str := SizeByte64(value, includeSign)
	if value < 0 {
		str = decreaseStyle.Render(str)
	} else if value > 0 {
		str = increaseStyle.Render(str)
	} else {
		str = unchangedStyle.Render(str)
	}
	return str
}
