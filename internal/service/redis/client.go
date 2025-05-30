package redis

import (
	"context"
	"net"
	"strconv"
	"strings"

	rediscli "github.com/redis/go-redis/v9"
)

const (
	redisRoleMaster = "role:master"
)

type ConnectionInfo struct {
	IP       string
	Port     string
	Password string
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
		Addr:     net.JoinHostPort(s.connectionInfo.IP, s.connectionInfo.Port),
		Password: s.connectionInfo.Password,
		DB:       0,
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

	cmd = rediscli.NewBoolCmd(ctx, "SENTINEL", "MONITOR", masterGroupName, master.IP, master.Port, quorum)
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
