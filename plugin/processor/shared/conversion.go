package shared

import "fmt"

func SizeByte(v float64) string {
	return SizeByte64(float64(v))
}

func SizeByte64(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-%s", SizeByte64(-v))
	}

	if v < 1024 {
		return fmt.Sprintf("%.0f Bytes", v)
	}
	v = v / 1024
	if v < 1024 {
		return fmt.Sprintf("%.1f KB", v)
	}
	v = v / 1024
	if v < 1024 {
		return fmt.Sprintf("%.1f MB", v)
	}
	v = v / 1024
	return fmt.Sprintf("%.1f GB", v)
}

func SizeByte64WithStyle(value float64) string {
	str := SizeByte64(value)
	if value < 0 {
		str = decreaseStyle.Render(str)
	} else if value > 0 {
		str = increaseStyle.Render(str)
	} else {
		str = unchangedStyle.Render(str)
	}
	return str
}
