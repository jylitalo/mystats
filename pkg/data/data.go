package data

func Coalesce[T comparable](args ...T) T {
	var def T
	for _, arg := range args {
		if arg != def {
			return arg
		}
	}
	return def
}
