package utils

func ToBool(a any) (bool, bool) {
	v, ok := a.(bool)
	return v, ok
}

func ToString(a any) string {
	v, ok := a.(string)
	if !ok {
		return ""
	}
	return v
}

func ToInt(a any) (int, bool) {
	v, ok := a.(int)
	return v, ok
}
