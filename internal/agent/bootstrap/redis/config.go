package bootstrap

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	agentutil "github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/util"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/consts"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	"github.com/Showmax/go-fqdn"
)

// defaultRedisConfig from https://github.com/OT-CONTAINER-KIT/redis/blob/master/redis.conf
// tcp-keepalive is lowered from the Redis default (300s) to 60s so that dead
// peer connections are detected faster, which helps the cluster gossip layer
// converge sooner after pod restarts with new IPs.
const defaultRedisConfig = `
bind 0.0.0.0 ::
tcp-backlog 511
timeout 0
tcp-keepalive 60
daemonize no
supervised no
pidfile /var/run/redis.pid
`

// GenerateConfig generates Redis configuration file
func GenerateConfig() error {
	confPath := util.CoalesceEnv1("REDIS_CONFIG_FILE", "/etc/redis/redis.conf")
	cfg := agentutil.NewConfig(confPath, defaultRedisConfig)

	var (
		persistenceEnabled = util.CoalesceEnv1("PERSISTENCE_ENABLED", "false")
		dataDir            = util.CoalesceEnv1("DATA_DIR", "/data")
		nodeConfDir        = util.CoalesceEnv1("NODE_CONF_DIR", "/node-conf")
		externalConfigFile = util.CoalesceEnv1("EXTERNAL_CONFIG_FILE", "/etc/redis/external.conf.d/redis-additional.conf")
		redisMajorVersion  = util.CoalesceEnv1("REDIS_MAJOR_VERSION", "v7")
		redisPort          = util.CoalesceEnv1("REDIS_PORT", "6379")
		nodeport           = util.CoalesceEnv1("NODEPORT", "false")
		tlsMode            = util.CoalesceEnv1("TLS_MODE", "false")
		clusterMode        = util.CoalesceEnv1("SETUP_MODE", "standalone")
		aclMode            = util.CoalesceEnv1("ACL_MODE", "")
		aclFilePath        = util.CoalesceEnv1("ACL_FILE_PATH", "/etc/redis/user.acl")
		expandExternal     = util.CoalesceEnv1(consts.ENV_KEY_EXPAND_EXTERNAL_CONFIG, "false")
	)

	if val, ok := util.CoalesceEnv("REDIS_PASSWORD", ""); ok && val != "" {
		cfg.Append("masterauth", val)
		cfg.Append("requirepass", val)
		cfg.Append("protected-mode", "yes")
	} else {
		fmt.Println("Redis is running without password which is not recommended")
		cfg.Append("protected-mode", "no")
	}

	if clusterMode == "cluster" {
		nodeConfPath := filepath.Join(nodeConfDir, "nodes.conf")

		// cluster-node-timeout is raised from 5000ms to 15000ms (configurable
		// via CLUSTER_NODE_TIMEOUT) to give gossip time to converge after
		// pod restarts before marking nodes as failed.
		clusterNodeTimeout := util.CoalesceEnv1("CLUSTER_NODE_TIMEOUT", "15000")

		cfg.Append("cluster-enabled", "yes")
		cfg.Append("cluster-node-timeout", clusterNodeTimeout)
		cfg.Append("cluster-require-full-coverage", "no")
		cfg.Append("cluster-migration-barrier", "1")
		cfg.Append("cluster-config-file", nodeConfPath)

		if ip, err := util.GetLocalIP(); err != nil {
			log.Printf("Warning: Failed to get local IP: %v", err)
		} else {
			_, err = updateMyselfIP(nodeConfPath, strings.TrimSpace(ip))
			if err != nil {
				log.Printf("Warning: Failed to update nodes.conf: %v", err)
			}
		}

		var err error
		var clusterAnnounceIP string
		if nodeport == "true" {
			clusterAnnounceIP = os.Getenv("HOST_IP")
		} else {
			clusterAnnounceIP, err = util.GetLocalIP()
			if err != nil {
				log.Printf("Warning: Failed to get local IP: %v", err)
			}
		}
		if clusterAnnounceIP != "" {
			cfg.Append("cluster-announce-ip", clusterAnnounceIP)
		}
		if redisMajorVersion == "v7" {
			fqdnName, err := fqdn.FqdnHostname()
			if err != nil {
				log.Printf("Warning: Failed to get FQDN: %v", err)
			} else {
				cfg.Append("cluster-announce-hostname", fqdnName)
			}
		}
	} else {
		fmt.Println("Setting up redis in standalone mode")
	}

	if tlsMode == "true" {
		cfg.Append("tls-cert-file", util.CoalesceEnv1("REDIS_TLS_CERT", ""))
		cfg.Append("tls-key-file", util.CoalesceEnv1("REDIS_TLS_CERT_KEY", ""))
		if caCert := util.CoalesceEnv1("REDIS_TLS_CA_CERT", ""); caCert != "" {
			cfg.Append("tls-ca-cert-file", caCert)
		}
		cfg.Append("tls-auth-clients", "optional")
		cfg.Append("tls-replication", "yes")

		if clusterMode == "cluster" {
			cfg.Append("tls-cluster", "yes")
			if redisMajorVersion == "v7" && nodeport == "false" {
				cfg.Append("cluster-preferred-endpoint-type", "hostname")
			}
		}
	} else {
		fmt.Println("Running without TLS mode")
	}

	if aclMode == "true" {
		fmt.Println("ACL_MODE is true, modifying ACL file path to", aclFilePath)
		cfg.Append("aclfile", aclFilePath)
	} else {
		fmt.Println("ACL_MODE is not true, skipping ACL file modification")
	}

	if persistenceEnabled == "true" {
		cfg.Append("save", "900 1")
		cfg.Append("save", "300 10")
		cfg.Append("save", "60 10000")
		cfg.Append("Appendonly", "yes")
		cfg.Append("Appendfilename", "\"Appendonly.aof\"")
		cfg.Append("dir", dataDir)
	} else {
		fmt.Println("Running without persistence mode")
	}

	if tlsMode == "true" {
		cfg.Append("port", "0")
		cfg.Append("tls-port", redisPort)
	} else {
		cfg.Append("port", redisPort)
	}

	if nodeport == "true" {
		podHostname, _ := os.Hostname()
		announcePortVar := "announce_port_" + strings.ReplaceAll(podHostname, "-", "_")
		announceBusPortVar := "announce_bus_port_" + strings.ReplaceAll(podHostname, "-", "_")

		// Get environment variables
		clusterAnnouncePort := os.Getenv(announcePortVar)
		clusterAnnounceBusPort := os.Getenv(announceBusPortVar)

		if clusterAnnouncePort != "" {
			cfg.Append("cluster-announce-port", clusterAnnouncePort)
			if tlsMode == "true" {
				cfg.Append("cluster-announce-tls-port", clusterAnnouncePort)
			}
		}
		if clusterAnnounceBusPort != "" {
			cfg.Append("cluster-announce-bus-port", clusterAnnounceBusPort)
		}
	}
	if maxMemory := util.CoalesceEnv1(consts.ENV_KEY_REDIS_MAX_MEMORY, ""); maxMemory != "" {
		cfg.Append("maxmemory", maxMemory)
	}
	// External configuration defined by user at the end. The file is mounted
	// read-only from a ConfigMap and may contain ${VAR} placeholders (e.g.
	// `requirepass ${REDIS_PASSWORD}`); Redis does not expand env vars in its
	// config, so when EXPAND_EXTERNAL_CONFIG=true we expand them here against
	// the container env and include the expanded copy. Without this the literal
	// `${REDIS_PASSWORD}` would be loaded verbatim and, since the include is
	// processed last, would override the operator-managed requirepass/masterauth
	// set above. Gated off by default to preserve historical behaviour.
	if _, err := os.Stat(externalConfigFile); err == nil {
		includePath := externalConfigFile
		if expandExternal == "true" {
			expandedPath, err := expandExternalConfig(externalConfigFile, confPath)
			if err != nil {
				log.Printf("Warning: Failed to expand external config %s, including verbatim: %v", externalConfigFile, err)
			} else {
				includePath = expandedPath
			}
		}
		cfg.Append("include", includePath)
	}
	return cfg.Commit()
}

// expandExternalConfig reads the user-provided external config, expands ${VAR}
// / $VAR references against the container environment, and writes the result
// next to the generated redis.conf. It returns the path to include.
//
// The expanded file is written to the redis.conf directory (an emptyDir shared
// by the init and main containers), NOT next to the source: the source is a
// read-only ConfigMap mount, and DATA_DIR is not guaranteed to be mounted in
// the bootstrap init container. The redis.conf dir is always writable and
// mounted in both containers, so the include path resolves at Redis startup.
func expandExternalConfig(externalConfigFile, confPath string) (string, error) {
	raw, err := os.ReadFile(externalConfigFile)
	if err != nil {
		return "", err
	}
	expanded := os.Expand(string(raw), func(key string) string {
		return util.CoalesceEnv1(key, "")
	})

	expandedPath := filepath.Join(filepath.Dir(confPath), "redis-additional.expanded.conf")
	if err := os.WriteFile(expandedPath, []byte(expanded), 0o644); err != nil {
		return "", fmt.Errorf("write expanded config to %s: %v", expandedPath, err)
	}
	return expandedPath, nil
}

func updateMyselfIP(nodesConfPath, newIP string) (updated []byte, err error) {
	raw, err := os.ReadFile(nodesConfPath)
	if err != nil {
		return nil, err
	}
	ipRe := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	var out bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	changed := false

	for scanner.Scan() {
		line := scanner.Text()
		if bytes.Contains([]byte(line), []byte("myself")) {
			replaced := ipRe.ReplaceAllString(line, newIP)
			if replaced != line {
				changed = true
				line = replaced
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if changed {
		return out.Bytes(), os.WriteFile(nodesConfPath, out.Bytes(), 0o644)
	}
	return nil, nil
}
