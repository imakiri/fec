package observer

type option[T any] func(t *T)

type Reporter interface {
	Report(total, delta uint64)
}
