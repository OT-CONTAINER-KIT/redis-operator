package controller

import (
	"github.com/iamabhishek-dubey/redis-operator/pkg/controller/redis"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, redis.Add)
}
