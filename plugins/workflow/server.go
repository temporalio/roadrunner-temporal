package workflow

import (
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

const RRMode = "temporal/workflows"

type Server struct {
	temporal temporal.Temporal
	wFactory factory.WorkerFactory

	// currently active worker pool (can be replaced at runtime)
	//pool *activityPool
}

// logger dep also
func (s *Server) Init(temporal temporal.Temporal, wFactory factory.WorkerFactory) error {
	s.temporal = temporal
	s.wFactory = wFactory
	return nil
}
