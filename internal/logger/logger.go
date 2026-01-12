package logger

import (
	"log"
	"sync"

	"go.uber.org/zap"
)

var (
	logg *zap.Logger
	once sync.Once
)

func Instance() *zap.Logger {
	once.Do(func() {
		var err error
		logg, err = zap.NewProduction()
		if err != nil {
			log.Fatalf("can't initialize zap logger: %v", err)
		}
	})
	return logg
}
