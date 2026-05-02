package safego

import (
	"log"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
)

var goroutinePanics = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "goroutine_panics_total",
		Help: "Total number of recovered panics in spawned goroutines.",
	},
	[]string{"location"},
)

func init() {
	prometheus.MustRegister(goroutinePanics)
}

func Go(location string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in goroutine [%s]: %v\n%s", location, r, debug.Stack())
				goroutinePanics.WithLabelValues(location).Inc()
			}
		}()
		fn()
	}()
}
