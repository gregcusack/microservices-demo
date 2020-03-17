package faultservice

import "fmt"

type logger struct {
	server FaultService_ExperimentServer
}

func newLogger(server FaultService_ExperimentServer) *logger {
	return &logger{server: server}
}

func (l *logger) Infof(template string, args ...interface{}) {
	sugar.Infof(template, args...)
	l.server.Send(&InfoMsg{Info: fmt.Sprintf(template+"\n", args...)})
}
