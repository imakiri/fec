package fec

import (
	"fmt"
)

type Reporter struct {
	header string
	writer chan []byte
}

func NewReporter(writer chan []byte, header string) (*Reporter, error) {
	var reporter = new(Reporter)
	reporter.header = header
	reporter.writer = writer
	return reporter, nil
}

func (r Reporter) Report(total, delta uint64) {
	r.writer <- []byte(fmt.Sprintf("[%s] delta: %6d kbit/sec, total: %6d mbit\n", r.header, delta*8/(1<<10), total*8/(1<<20)))
}
