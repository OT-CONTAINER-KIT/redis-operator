// recovery-race demonstrates whether a newer Sentinel election is overwritten
// after the authority recovery caller has already checked an older report.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
)

func readCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
	if err != nil {
		return nil, err
	}
	args := make([]string, count)
	for i := range args {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "$")))
		if err != nil {
			return nil, err
		}
		value := make([]byte, size+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, err
		}
		args[i] = string(value[:size])
	}
	return args, nil
}

func report(ip, epoch string) string {
	fields := []string{"ip", ip, "port", "6379", "flags", "master", "config-epoch", epoch}
	response := fmt.Sprintf("*%d\r\n", len(fields))
	for _, field := range fields {
		response += fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)
	}
	return response
}

func main() {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	var mutex sync.Mutex
	queries, removes, monitors := 0, 0, 0
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					args, err := readCommand(reader)
					if err != nil {
						return
					}
					response := "+OK\r\n"
					mutex.Lock()
					if strings.EqualFold(args[0], "hello") {
						response = "%2\r\n+server\r\n+redis\r\n+version\r\n+8.4.0\r\n"
					} else if strings.EqualFold(args[0], "sentinel") && len(args) > 1 {
						switch strings.ToLower(args[1]) {
						case "master":
							queries++
							response = report("192.0.2.2", "4")
							if queries >= 3 {
								response = report("192.0.2.3", "6")
							}
						case "remove":
							removes++
						case "monitor":
							monitors++
						}
					}
					mutex.Unlock()
					if _, err := io.WriteString(conn, response); err != nil {
						return
					}
				}
			}()
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	svc := redis.NewClient().Connect(&redis.ConnectionInfo{Host: host, Port: port})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// The caller first observes B/4, then explicitly rechecks B/4.
	for i := 0; i < 2; i++ {
		state, err := svc.GetSentinelMaster(ctx, "myMaster")
		if err != nil || state.Epoch != "4" {
			panic(fmt.Sprintf("scripted report failed: %v", err))
		}
	}
	// Before the generic helper queries again, the peer learns C/6.
	err = svc.SentinelMonitorIfAbsent(ctx, &redis.ConnectionInfo{Host: "192.0.2.1", Port: "6379"}, "myMaster", "2")
	mutex.Lock()
	defer mutex.Unlock()
	result := map[string]interface{}{"queries": queries, "remove_commands": removes, "monitor_commands": monitors, "overwrote_newer_election": removes > 0 && monitors > 0, "operation_error": err != nil}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
	if removes > 0 || monitors > 0 {
		os.Exit(1)
	}
}
