package main

import (
	"fmt"

	_ "github.com/prometheus/client_golang/prometheus"
	//_ "github.com/prometheus/client_golang/prometheus/promhttp"
	//_ "github.com/prometheus/client_golang/prometheus/promauto"
)

/*
import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func recordMetrics() {
	go func() {
		for {
			opsProcessed.Inc()
			time.Sleep(2 * time.Second)
		}
	}()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
)
*/

func main() {
	//if false {
	//	recordMetrics()
	//
	//	http.Handle("/metrics", promhttp.Handler())
	//	http.ListenAndServe(":2112", nil)
	//}

	fmt.Println("It works!")
}
