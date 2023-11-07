package reporter

import "log"

type StdLog struct {
	format string
}

func NewStdLog(format string) StdLog {
	return StdLog{format: format}
}

func (c StdLog) Report(total, delta uint64) {
	log.Printf(c.format, total, delta)
}
