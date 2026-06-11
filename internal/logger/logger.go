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

func Init(development bool, level string) {
	once.Do(func() {
		var cfg zap.Config
		if development {
			cfg = zap.NewDevelopmentConfig()
		} else {
			cfg = zap.NewProductionConfig()
		}
		if level != "" {
			lvl, err := zap.ParseAtomicLevel(level)
			if err != nil {
				log.Fatalf("can't parse log level %q: %v", level, err)
			}
			cfg.Level = lvl
		}

		var err error
		logg, err = cfg.Build()
		if err != nil {
			log.Fatalf("can't initialize zap logger: %v", err)
		}
	})
}

func Instance() *zap.Logger {
	if logg == nil {
		log.Fatal("logger not initialized: call logger.Init first")
	}
	return logg
}
