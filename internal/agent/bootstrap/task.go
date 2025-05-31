package bootstrap

import (
	redisbootstrap "github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/bootstrap/redis"
	sentinelbootstrap "github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/bootstrap/sentinel"
)

type Task struct {
	sentinel bool
}

func NewTask(sentinel bool) *Task {
	return &Task{
		sentinel: sentinel,
	}
}

func (t *Task) Run() error {
	if t.sentinel {
		return sentinelbootstrap.GenerateConfig()
	}
	return redisbootstrap.GenerateConfig()
}
