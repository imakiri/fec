package reporter

type Reporter func(total, delta uint64)

func (c Reporter) Report(total, delta uint64) {
	c(total, delta)
}
