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
	service.client = &client{
		new: func(ctx context.Context) (*rediscli.Client, error) {
			opts := &rediscli.Options{
				Addr:     net.JoinHostPort(info.IP, info.Port),
				Password: info.Password,
				DB:       0,
			}
			rClient := rediscli.NewClient(opts)
			defer rClient.Close()
			return rClient, nil
		},
	}
	return service
}

type client struct {
	new func(ctx context.Context) (*rediscli.Client, error)
}

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
	client *client
}

func NewService() Service {
	return &service{}
}

func (c *service) SentinelReset(ctx context.Context, masterGroupName string) error {
	client, err := c.client.new(ctx)
	if err != nil {
		return err
	}
	cmd := rediscli.NewStringCmd(ctx, "SENTINEL", "RESET", masterGroupName)
	err = client.Process(ctx, cmd)
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

	client, err := c.client.new(ctx)
	if err != nil {
		return err
	}
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
	client, err := c.client.new(ctx)
	if err != nil {
		return false, err
	}
	result, err := client.Info(ctx, "replication").Result()
	if err != nil {
		return false, err
	}
	return strings.Contains(result, redisRoleMaster), nil
}

func (c *service) GetAttachedReplicaCount(ctx context.Context) (int, error) {
	client, err := c.client.new(ctx)
	if err != nil {
		return 0, err
	}
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
