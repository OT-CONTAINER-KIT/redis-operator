// client-probe exercises persistent direct-Service and Sentinel-aware connections
// with synthetic data. It emits counts only, never connection credentials.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	address := flag.String("address", "redis-replication-master:6379", "direct primary Service")
	sentinels := flag.String("sentinels", "redis-sentinel-sentinel:26379", "Sentinel addresses")
	duration := flag.Duration("duration", 90*time.Second, "observation window")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	start := time.Now()
	var mutex sync.Mutex
	counts := map[string]int{}
	errorCategories := map[string]int{}
	errorTypes := map[string]int{}
	lastSuccess := map[string]float64{}
	record := func(kind string, err error) {
		mutex.Lock()
		defer mutex.Unlock()
		outcome := "ok"
		if err != nil {
			outcome = "error"
			if strings.Contains(err.Error(), "READONLY") {
				outcome = "readonly"
			}
			category := "other"
			errorTypes[kind+"_"+fmt.Sprintf("%T", err)]++
			var network net.Error
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				category = "context_deadline"
			case errors.Is(err, context.Canceled):
				category = "context_canceled"
			case errors.As(err, &network) && network.Timeout():
				category = "network_timeout"
			case strings.Contains(err.Error(), "connection pool timeout"):
				category = "pool_timeout"
			case strings.Contains(err.Error(), "READONLY"):
				category = "readonly"
			case strings.Contains(err.Error(), "connection refused"):
				category = "connection_refused"
			case strings.Contains(err.Error(), "EOF"):
				category = "eof"
			}
			errorCategories[kind+"_"+category]++
			for _, code := range []string{"ERR", "READONLY", "NOREPLICAS", "MASTERDOWN", "MISCONF", "NOSCRIPT", "BUSY", "OOM", "WRONGPASS", "NOAUTH", "LOADING", "CLUSTERDOWN", "MOVED", "ASK"} {
				if strings.HasPrefix(err.Error(), code+" ") {
					errorCategories[kind+"_server_"+code]++
				}
			}
		} else {
			lastSuccess[kind] = time.Since(start).Seconds()
		}
		counts[kind+"_"+outcome]++
	}
	options := &redis.Options{Addr: *address, PoolSize: 4, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second}
	direct := redis.NewClient(options)
	defer direct.Close()
	sentinel := redis.NewFailoverClient(&redis.FailoverOptions{MasterName: "myMaster", SentinelAddrs: strings.Split(*sentinels, ","), PoolSize: 4, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	defer sentinel.Close()
	pubsub := direct.Subscribe(ctx, "local-authority-probe-channel")
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		fmt.Println(`{"probe_start_failed":true}`)
		os.Exit(1)
	}
	var workers sync.WaitGroup
	run := func(work func()) {
		workers.Add(1)
		go func() { defer workers.Done(); work() }()
	}
	for name, client := range map[string]*redis.Client{"direct": direct, "sentinel": sentinel} {
		for worker := 0; worker < 4; worker++ {
			kind, conn := name, client
			run(func() {
				for ctx.Err() == nil {
					record(kind, conn.Incr(ctx, "local-authority-probe-counter").Err())
					time.Sleep(200 * time.Millisecond)
				}
			})
		}
	}
	// A separately dialled Service connection distinguishes routing failures
	// from persistence/reuse of an old pool's connections.
	run(func() {
		for ctx.Err() == nil {
			fresh := redis.NewClient(&redis.Options{Addr: *address, PoolSize: 1, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
			record("fresh_service", fresh.Incr(ctx, "local-authority-fresh-counter").Err())
			_ = fresh.Close()
			time.Sleep(time.Second)
		}
	})
	run(func() {
		channel := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-channel:
				if !ok {
					return
				}
				record("pubsub", nil)
			}
		}
	})
	run(func() {
		for ctx.Err() == nil {
			record("publish", sentinel.Publish(ctx, "local-authority-probe-channel", "synthetic-payload").Err())
			record("push", sentinel.LPush(ctx, "local-authority-probe-list", "synthetic-item").Err())
			time.Sleep(300 * time.Millisecond)
		}
	})
	run(func() {
		for ctx.Err() == nil {
			_, err := direct.BRPop(ctx, time.Second, "local-authority-probe-list").Result()
			if err != redis.Nil {
				record("blocking", err)
			}
			if err != nil && err != redis.Nil {
				// A cached dial failure can return immediately. Do not turn a
				// reconnect check into an unbounded CPU/connection retry storm.
				time.Sleep(200 * time.Millisecond)
			}
		}
	})
	fmt.Println(`{"persistent_clients_started":true}`)
	workers.Wait()
	mutex.Lock()
	defer mutex.Unlock()
	passed := true
	for _, kind := range []string{"direct", "sentinel", "pubsub", "blocking"} {
		if lastSuccess[kind] < duration.Seconds()-10 {
			passed = false
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"passed": passed, "counts": counts, "error_categories": errorCategories, "error_types": errorTypes, "direct_pool": direct.PoolStats(), "last_success_seconds": lastSuccess, "duration_seconds": duration.Seconds()}); err != nil {
		panic(err)
	}
	if !passed {
		os.Exit(1)
	}
}
