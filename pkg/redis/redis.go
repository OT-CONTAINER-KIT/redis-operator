package redis

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-redis/redis"
	"github.com/spf13/cast"
)

const (
	Port = 6379

	// MinimumFailoverSize sets the minimum desired size of Redis replication.
	// It reflects a simple master - replica pair.
	// Due to the highly volatile nature of Kubernetes environments
	// it is better to keep at least 3 instances and feel free to lose one instance for whatever reason.
	// It is especially useful for scenarios when there is no need or permission to use persistent storage.
	// In such cases it is safe to run Redis replication failover and the risk of losing data is minimal.
	MinimumFailoverSize = 2

	// INFO REPLICATION fields
	RoleMaster  = "role:master"
	RoleReplica = "role:slave"

	// master-specific fields
	connectedReplicas = "connected_slaves"
	masterReplOffset  = "master_repl_offset"

	// replica-specific fields
	replicaPriority   = "slave_priority"
	replicationOffset = "slave_repl_offset"
	masterHost        = "master_host"
	masterPort        = "master_port"
	masterLinkStatus  = "master_link_status"

	// DefaultFailoverTimeout sets the maximum timeout for an exponential backoff timer
	DefaultFailoverTimeout = 5 * time.Second
)

var (
	infoReplicationRe = buildInfoReplicationRe()
)

// buildInfoReplicationRe is a helper function to build a regexp for parsing INFO REPLICATION output
func buildInfoReplicationRe() *regexp.Regexp {
	var b strings.Builder
	defer b.Reset()
	// start from setting the multi-line flag
	b.WriteString(`(?m)`)

	// IPv4 address regexp
	addrRe := `((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`

	// templates for simple fields
	numTmpl := `^%s:\d+\s*?$`
	strTmpl := `^%s:\w+\s*?$`

	// build them all up
	for name, tmpl := range map[string]string{
		// master-specific fields
		connectedReplicas: numTmpl,
		masterReplOffset:  numTmpl,

		// replica-specific fields
		replicaPriority:   numTmpl,
		replicationOffset: numTmpl,
		masterHost:        fmt.Sprintf(`^%%s:%s\s*?$`, addrRe),
		masterPort:        numTmpl,
		masterLinkStatus:  strTmpl,
	} {
		fmt.Fprintf(&b, tmpl, name)
		fmt.Fprint(&b, "|")
	}
	// replica regexp is the most complex of all
	fmt.Fprintf(&b, `^slave\d+:ip=%s,port=\d{1,5},state=\w+,offset=\d+,lag=\d+\s*?$`, addrRe)
	return regexp.MustCompile(b.String())
}

// Rediser defines the Redis methods
type Rediser interface {
	Ping() error

	replicaOf(master Address) error
	refresh(info string) error
	getInfo() (string, error)
}

// Failover speaks for itself
type Failover interface {
	Reconfigure() error
	SelectMaster() *Redis
	Refresh() error
	Disconnect()

	promoteReplicaToMaster() (*Redis, error)
	reconfigureAsReplicasOf(master Address) error
}

// Address represents the Host:Port pair of a Redis instance
type Address struct {
	Host string
	Port string
}

// Redis struct includes a subset of fields returned by INFO
type Redis struct {
	Address

	Role              string
	ReplicationOffset int

	// master-specific fields
	ConnectedReplicas int
	Replicas          Redises

	// replica-specific fields
	ReplicaPriority  int
	MasterHost       string
	MasterPort       string
	MasterLinkStatus string

	conn *redis.Client
}

type Redises []Redis

// sort.Interface implementation for Redises.
// Allows to choose an instance with a lesser priority and higher replication offset.
// Note that this assumes that Redises don't have replicas with ReplicaPriority == 0
func (instances Redises) Len() int      { return len(instances) }
func (instances Redises) Swap(i, j int) { instances[i], instances[j] = instances[j], instances[i] }
func (instances Redises) Less(i, j int) bool {
	// choose a replica with less replica priority
	// choose a bigger replication offset otherwise
	if instances[i].ReplicaPriority == instances[j].ReplicaPriority {
		return instances[i].ReplicationOffset > instances[j].ReplicationOffset
	}
	return instances[i].ReplicaPriority < instances[j].ReplicaPriority
}

// strict implementation check
var _ Rediser = (*Redis)(nil)
var _ Failover = (*Redises)(nil)
var _ sort.Interface = (*Redises)(nil)

func (a Address) String() string {
	return fmt.Sprintf("%s:%s", a.Host, a.Port)
}

// Ping errs on error if PING failed
func (r *Redis) Ping() error {
	return r.conn.Ping().Err()
}

// replicaOf changes the replication settings of a replica on the fly
func (r *Redis) replicaOf(master Address) (err error) {
	// promote replica to master
	if master == (Address{}) {
		master.Host = "NO"
		master.Port = "ONE"
	}

	/* In order to send REPLICAOF in a safe way, we send a transaction performing
	 * the following tasks:
	 * 1) Reconfigure the instance according to the specified host/port params.
	 * 2) Disconnect all clients (but this one sending the command) in order
	 *    to trigger the ask-master-on-reconnection protocol for connected
	 *    clients.
	 *
	 * Note that we don't check the replies returned by commands, since we
	 * will observe instead the effects in the next INFO output. */
	_, err = r.conn.TxPipelined(func(pipe redis.Pipeliner) error {
		pipe.SlaveOf(master.Host, master.Port)
		pipe.ClientKillByFilter("TYPE", "NORMAL")
		return nil
	})

	return err
}

func (r *Redis) getInfo() (info string, err error) {
	info, err = r.conn.Info("replication").Result()
	if err != nil {
		return "", fmt.Errorf("getting info replication failed for %s: %s", r.Address, err)
	}
	return
}

// refresh parses the instance info and updates the instance fields appropriately
func (r *Redis) refresh(info string) error {
	// parse info replication answer
	switch {
	case strings.Contains(info, RoleMaster):
		r.Role = RoleMaster
	case strings.Contains(info, RoleReplica):
		r.Role = RoleReplica
	default:
		return errors.New("the role is wrong")
	}

	// parse all other attributes
	for _, parsed := range infoReplicationRe.FindAllString(info, -1) {
		switch s := strings.TrimSpace(parsed); {
		// master-specific
		case r.Role == RoleMaster && strings.HasPrefix(s, connectedReplicas):
			r.ConnectedReplicas = cast.ToInt(strings.Split(s, ":")[1])
		case r.Role == RoleMaster && strings.HasPrefix(s, masterReplOffset):
			r.ReplicationOffset = cast.ToInt(strings.Split(s, ":")[1])
		case r.Role == RoleMaster && strings.HasPrefix(s, "slave"):
			replica := Redis{}
			for _, field := range strings.Split(strings.Split(s, ":")[1], ",") {
				switch {
				case strings.HasPrefix(field, "ip="):
					replica.Host = strings.Split(field, "=")[1]
				case strings.HasPrefix(field, "port="):
					replica.Port = strings.Split(field, "=")[1]
				case strings.HasPrefix(field, "offset="):
					replica.ReplicationOffset = cast.ToInt(strings.Split(field, "=")[1])
				}
			}
			r.Replicas = append(r.Replicas, replica)

		// replica-specific
		case r.Role == RoleReplica && strings.HasPrefix(s, replicaPriority):
			r.ReplicaPriority = cast.ToInt(strings.Split(s, ":")[1])
		case r.Role == RoleReplica && strings.HasPrefix(s, replicationOffset):
			r.ReplicationOffset = cast.ToInt(strings.Split(s, ":")[1])
		case r.Role == RoleReplica && strings.HasPrefix(s, masterHost):
			r.MasterHost = strings.Split(s, ":")[1]
		case r.Role == RoleReplica && strings.HasPrefix(s, masterLinkStatus):
			r.MasterLinkStatus = strings.Split(s, ":")[1]
		case r.Role == RoleReplica && strings.HasPrefix(s, masterPort):
			r.MasterPort = strings.Split(s, ":")[1]
		}
	}
	return nil
}

// Reconfigure checks the state of the Redis replication and tries to fix/initially set the state.
// There should be only one master. All other instances should report the same master.
// Working master serves as a source of truth. It means that only those replicas who are not reported by master
// as its replicas will be reconfigured.
func (instances Redises) Reconfigure() (err error) {
	// nothing to do here
	if len(instances) == 0 {
		return nil
	}

	var master *Redis
	var replicas Redises
	master = instances.SelectMaster()

	// we've lost the master, promote a replica to master role
	if master == nil {
		var candidates Redises
		// filter out non-replicas
		for i := range instances {
			if instances[i].Role == RoleReplica && instances[i].ReplicaPriority != 0 {
				candidates = append(candidates, instances[i])
			}
		}
		master, err = candidates.promoteReplicaToMaster()
		if err != nil {
			return err
		}
	}

	// connectedReplicas will be needed to compile a slice of orphaned(not connected to current master) instances
	connectedReplicas := map[Address]struct{}{}
	for _, replica := range master.Replicas {
		connectedReplicas[replica.Address] = struct{}{}
	}

	for i := range instances {
		if _, there := connectedReplicas[instances[i].Address]; instances[i].Address != master.Address && !there {
			replicas = append(replicas, instances[i])
		}
	}

	// configure replicas
	return replicas.reconfigureAsReplicasOf(master.Address)
}

// SelectMaster chooses any working master in case of a working replication or any other master otherwise.
// Working master in this case is a master with at least one replica connected.
func (instances Redises) SelectMaster() *Redis {
	// normal state. we have a working replication with the master being online
	for _, i := range instances {
		// filter out replicas since they can also have their own replicas...
		if i.Role == RoleReplica {
			continue
		}

		// we've found a working master
		if i.ConnectedReplicas > 0 {
			return &i
		}
	}

	// If we have at least one replica it means
	// we've lost the current master and need to promote a replica to a master
	for _, i := range instances {
		if i.Role == RoleReplica && i.ReplicaPriority != 0 {
			return nil
		}
	}

	// This is supposed to be an initial state.
	// When you roll out a bunch of Redis instances initially they are all standalone masters.
	// In this case we are free to choose the first one.
	if len(instances) > 0 {
		return &instances[0]
	}

	return nil
}

// Refresh fetches and refreshes info for all instances
func (instances Redises) Refresh() error {
	var wg sync.WaitGroup
	instanceCount := len(instances)
	ch := make(chan string, instanceCount)
	wg.Add(instanceCount)

	for i := range instances {
		go func(i *Redis, wg *sync.WaitGroup) {
			defer wg.Done()
			info, err := i.getInfo()
			if err != nil {
				ch <- fmt.Sprintf("%s: %s", i.Address, err)
				return
			}
			if err := i.refresh(info); err != nil {
				ch <- fmt.Sprintf("%s: %s", i.Address, err)
				return
			}
		}(&instances[i], &wg)
	}
	wg.Wait()
	close(ch)

	if len(ch) > 0 {
		var b strings.Builder
		defer b.Reset()
		for e := range ch {
			fmt.Fprintf(&b, "%s;", e)
		}
		return errors.New(b.String())
	}
	return nil
}

// Disconnect closes the connections and releases the resources
func (instances Redises) Disconnect() {
	for i := range instances {
		_ = instances[i].conn.Close()
	}
}

// promoteReplicaToMaster selects a replica for promotion and promotes it to master role
func (replicas Redises) promoteReplicaToMaster() (*Redis, error) {
	sort.Sort(replicas)
	promoted := &replicas[0]
	exponentialBackOff := backoff.NewExponentialBackOff()
	exponentialBackOff.MaxElapsedTime = DefaultFailoverTimeout

	if err := promoted.replicaOf(Address{}); err != nil {
		return nil, fmt.Errorf("could not promote replica %s to master: %s", promoted.Address, err)
	}

	// promote replica to master and wait until it reports itself as a master
	return promoted, backoff.Retry(func() error {
		info, err := promoted.getInfo()
		if err != nil {
			return err
		}
		if err := promoted.refresh(info); err != nil {
			return err
		}
		if promoted.Role != RoleMaster {
			return fmt.Errorf("still waiting for the replica %s to be promoted", promoted.Address)
		}
		return nil
	}, exponentialBackOff)
}

func (replicas Redises) reconfigureAsReplicasOf(master Address) error {
	// do it simultaneously for all replicas
	var wg sync.WaitGroup
	replicasCount := len(replicas)
	ch := make(chan string, replicasCount)
	wg.Add(replicasCount)

	for i := range replicas {
		go func(replica *Redis, wg *sync.WaitGroup) {
			defer wg.Done()

			if err := replica.replicaOf(master); err != nil {
				ch <- fmt.Sprintf("error reconfiguring replica %s: %v", replica.Address, err)
			}
		}(&replicas[i], &wg)
	}
	wg.Wait()
	close(ch)

	if len(ch) > 0 {
		var b strings.Builder
		defer b.Reset()
		for e := range ch {
			fmt.Fprintf(&b, "%s;", e)
		}
		return errors.New(b.String())
	}
	return nil
}

// NewInstances returns a new set of Redis instances.
// If redis-operator fails to ping and refresh any of the connected instances NewInstances will return an error.
func NewInstances(password string, addrs ...Address) (Redises, error) {
	rs := Redises{}
	for _, addr := range addrs {
		r := Redis{
			Address: addr,
			conn:    redis.NewClient(&redis.Options{Addr: addr.String(), Password: password}),
		}

		// check connection and add the instance if Ping succeeds
		if err := r.Ping(); err != nil {
			// TODO: handle -BUSY status
			_ = r.conn.Close()
			rs.Disconnect()
			return nil, fmt.Errorf("ping failed for %s: %s", r.Host, err)
		}
		rs = append(rs, r)
	}

	if err := rs.Refresh(); err != nil {
		rs.Disconnect()
		return nil, fmt.Errorf("refreshing Redis instances info failed: %s", err)
	}
	return rs, nil
}
