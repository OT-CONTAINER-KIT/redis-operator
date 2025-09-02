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
	SentinelReset(ctx context.Context, masterGroupName string) error
	GetClusterInfo(ctx context.Context) (*ClusterStatus, error)
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
		if strings.HasPrefix(line, "connected_slaves:") {
			count, err = strconv.Atoi(strings.TrimPrefix(line, "connected_slaves:"))
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
