package service

import (
	"testing"

	"mikvoc/internal/repository"
)

func TestNewSchedulerStartStop(t *testing.T) {
	p := NewPool()
	s := NewScheduler(p, repository.NewStore())
	s.Start()
	s.tick()
	s.Stop()
	s.Stop()
}
