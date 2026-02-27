package redis

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"strings"

	rediscli "github.com/redis/go-redis/v9"
)

const (
	redisRoleMaster = "role:master"
)

type ConnectionInfo struct {
	// Host is the IP address or hostname for connection
	Host string
	// Port is the port for redis or sentinel
	Port     string
	Password string
	// TLSConfig configuration, nil means TLS is disabled
	TLSConfig *tls.Config
}

// ClusterStatus cluster status information, including the number of assigned slots
type ClusterStatus struct {
	SlotsAssigned int // Total number of assigned slots
}

// GetAddress returns the connection address
func (c *ConnectionInfo) GetAddress() string {
	return net.JoinHostPort(c.Host, c.Port)
}

type Client interface {
	Connect(info *ConnectionInfo) Service
}

func (c *client) Connect(info *ConnectionInfo) Service {
	service := &service{}
	service.connectionInfo = info
	return service
}

type client struct{}

func NewClient() Client {
	return &client{}
}

type Service interface {
	IsMaster(ctx context.Context) (bool, error)
	GetAttachedReplicaCount(ctx context.Context) (int, error)
	SentinelMonitor(ctx context.Context, master *ConnectionInfo, masterGroupName, quorum string) error
	SentinelSet(ctx context.Context, masterGroupName, key, value string) error
	SentinelReset(ctx context.Context, masterGroupName string) error
	GetInfoSentinel(ctx context.Context) (*InfoSentinelResult, error)
	GetClusterInfo(ctx context.Context) (*ClusterStatus, error)
}

type InfoSentinelResult struct {
	Masters []SentinelMasterInfo
}

type SentinelMasterInfo struct {
	Name      string
	Status    string
	Address   string
	Slaves    int
	Sentinels int
}

type service struct {
	connectionInfo *ConnectionInfo
}

func NewService() Service {
	return &service{}
}

func (s *service) createClient() *rediscli.Client {
	if s.connectionInfo == nil {
		return nil
	}
	opts := &rediscli.Options{
		Addr:     s.connectionInfo.GetAddress(),
		Password: s.connectionInfo.Password,
		DB:       0,
	}
	if s.connectionInfo.TLSConfig != nil {
		opts.TLSConfig = s.connectionInfo.TLSConfig
	}
	return rediscli.NewClient(opts)
}

func (c *service) GetInfoSentinel(ctx context.Context) (*InfoSentinelResult, error) {
	client := c.createClient()
	if client == nil {
		return nil, nil
	}
	defer client.Close()

	result, err := client.Info(ctx, "sentinel").Result()
	if err != nil {
		return nil, err
	}

	info := &InfoSentinelResult{}

	// Parse sentinel section
	// Expected format: master0:name=myMaster,status=ok,address=10.233.93.209:6379,slaves=2,sentinels=1
	for _, line := range strings.Split(result, "\r\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "master") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			masterData := make(map[string]string)
			for _, pair := range strings.Split(parts[1], ",") {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					masterData[kv[0]] = kv[1]
				}
			}

			master := SentinelMasterInfo{
				Name:    masterData["name"],
				Status:  masterData["status"],
				Address: masterData["address"],
			}
			if slavesStr, ok := masterData["slaves"]; ok {
				if slaves, err := strconv.Atoi(slavesStr); err == nil {
					master.Slaves = slaves
				}
			}
			if sentinelsStr, ok := masterData["sentinels"]; ok {
				if sentinels, err := strconv.Atoi(sentinelsStr); err == nil {
					master.Sentinels = sentinels
				}
			}

			info.Masters = append(info.Masters, master)
		}
	}

	return info, nil
}

func (c *service) SentinelSet(ctx context.Context, masterGroupName, key, value string) error {
	client := c.createClient()
	if client == nil {
		return nil
	}
	defer client.Close()

	cmd := rediscli.NewStringCmd(ctx, "SENTINEL", "SET", masterGroupName, key, value)
	err := client.Process(ctx, cmd)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return err
	}
	return nil
}

func (c *service) SentinelReset(ctx context.Context, masterGroupName string) error {
	client := c.createClient()
	if client == nil {
		return nil
	}
	defer client.Close()

	cmd := rediscli.NewStringCmd(ctx, "SENTINEL", "RESET", masterGroupName)
	err := client.Process(ctx, cmd)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return err
	}
	return nil
}

func (c *service) SentinelMonitor(ctx context.Context, master *ConnectionInfo, masterGroupName, quorum string) error {
	var (
		cmd *rediscli.BoolCmd
		err error
	)

	client := c.createClient()
	if client == nil {
		return nil
	}
	defer client.Close()

	masterCheckCmd := rediscli.NewSliceCmd(ctx, "SENTINEL", "MASTER", masterGroupName)
	if err = client.Process(ctx, masterCheckCmd); err == nil {
		if err = masterCheckCmd.Err(); err == nil {
			result, _ := masterCheckCmd.Result()
			var monitoredHost, monitoredPort string
			for i := 0; i+1 < len(result); i += 2 {
				key, ok := result[i].(string)
				if !ok {
					continue
				}
				var val string
				switch v := result[i+1].(type) {
				case string:
					val = v
				case []byte:
					val = string(v)
				default:
					continue
				}
				switch key {
				case "ip":
					monitoredHost = val
				case "port":
					monitoredPort = val
				}
			}
			if monitoredHost == master.Host && monitoredPort == master.Port {
				return nil
			}
		}
	}

	cmd = rediscli.NewBoolCmd(ctx, "SENTINEL", "REMOVE", masterGroupName)
	err = client.Process(ctx, cmd)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return err
	}

	cmd = rediscli.NewBoolCmd(ctx, "SENTINEL", "MONITOR", masterGroupName, master.Host, master.Port, quorum)
	err = client.Process(ctx, cmd)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return err
	}

	if master.Password != "" {
		cmd = rediscli.NewBoolCmd(ctx, "SENTINEL", "SET", masterGroupName, "auth-pass", master.Password)
		err = client.Process(ctx, cmd)
		if err != nil {
			return err
		}
		if err = cmd.Err(); err != nil {
			return err
		}
	}

	return nil
}

func (c *service) IsMaster(ctx context.Context) (bool, error) {
	client := c.createClient()
	if client == nil {
		return false, nil
	}
	defer client.Close()

	result, err := client.Info(ctx, "replication").Result()
	if err != nil {
		return false, err
	}
	return strings.Contains(result, redisRoleMaster), nil
}

func (c *service) GetAttachedReplicaCount(ctx context.Context) (int, error) {
	client := c.createClient()
	if client == nil {
		return 0, nil
	}
	defer client.Close()

	result, err := client.Info(ctx, "replication").Result()
	if err != nil {
		return 0, err
	}

	var count int
	for _, line := range strings.Split(result, "\r\n") {
		if value, found := strings.CutPrefix(line, "connected_slaves:"); found {
			count, err = strconv.Atoi(value)
			if err != nil {
				return 0, err
			}
			return count, nil
		}
	}
	return count, nil
}

// GetClusterInfo get cluster information by checking slot allocation
func (c *service) GetClusterInfo(ctx context.Context) (*ClusterStatus, error) {
	client := c.createClient()
	if client == nil {
		return nil, nil
	}
	defer client.Close()

	slots, err := client.ClusterSlots(ctx).Result()
	if err != nil {
		return nil, err
	}

	status := &ClusterStatus{
		SlotsAssigned: 0,
	}

	for _, slot := range slots {
		if slot.Start <= slot.End && len(slot.Nodes) > 0 {
			status.SlotsAssigned += slot.End - slot.Start + 1
		}
	}

	return status, nil
}
