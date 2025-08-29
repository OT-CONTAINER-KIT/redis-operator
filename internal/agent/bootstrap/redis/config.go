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
const defaultRedisConfig = `
bind 0.0.0.0 ::
tcp-backlog 511
timeout 0
tcp-keepalive 300
daemonize no
supervised no
pidfile /var/run/redis.pid
`

// GenerateConfig generates Redis configuration file
func GenerateConfig() error {
	cfg := agentutil.NewConfig("/etc/redis/redis.conf", defaultRedisConfig)

	var (
		persistenceEnabled, _ = util.CoalesceEnv("PERSISTENCE_ENABLED", "false")
		dataDir, _            = util.CoalesceEnv("DATA_DIR", "/data")
		nodeConfDir, _        = util.CoalesceEnv("NODE_CONF_DIR", "/node-conf")
		externalConfigFile, _ = util.CoalesceEnv("EXTERNAL_CONFIG_FILE", "/etc/redis/external.conf.d/redis-additional.conf")
		redisMajorVersion, _  = util.CoalesceEnv("REDIS_MAJOR_VERSION", "v7")
		redisPort, _          = util.CoalesceEnv("REDIS_PORT", "6379")
	)

	if val, ok := util.CoalesceEnv("REDIS_PASSWORD", ""); ok && val != "" {
		cfg.Append("masterauth", val)
		cfg.Append("requirepass", val)
		cfg.Append("protected-mode", "yes")
	} else {
		fmt.Println("Redis is running without password which is not recommended")
		cfg.Append("protected-mode", "no")
	}

	if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
		nodeConfPath := filepath.Join(nodeConfDir, "nodes.conf")

		cfg.Append("cluster-enabled", "yes")
		cfg.Append("cluster-node-timeout", "5000")
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
	} else {
		fmt.Println("Setting up redis in standalone mode")
	}

	if tlsMode, ok := util.CoalesceEnv("TLS_MODE", ""); ok && tlsMode == "true" {
		redisTLSCert, _ := util.CoalesceEnv("REDIS_TLS_CERT", "")
		redisTLSCertKey, _ := util.CoalesceEnv("REDIS_TLS_CERT_KEY", "")
		redisTLSCAKey, _ := util.CoalesceEnv("REDIS_TLS_CA_KEY", "")

		cfg.Append("tls-cert-file", redisTLSCert)
		cfg.Append("tls-key-file", redisTLSCertKey)
		cfg.Append("tls-ca-cert-file", redisTLSCAKey)
		cfg.Append("tls-auth-clients", "optional")
		cfg.Append("tls-replication", "yes")

		if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
			cfg.Append("tls-cluster", "yes")
			if redisMajorVersion == "v7" {
				cfg.Append("cluster-preferred-endpoint-type", "hostname")
			}
		}
	} else {
		fmt.Println("Running without TLS mode")
	}

	if aclMode, ok := util.CoalesceEnv("ACL_MODE", ""); ok && aclMode == "true" {
		cfg.Append("aclfile", "/etc/redis/user.acl")
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

	if tlsMode, ok := util.CoalesceEnv("TLS_MODE", ""); ok && tlsMode == "true" {
		cfg.Append("port", "0")
		cfg.Append("tls-port", redisPort)
	} else {
		cfg.Append("port", redisPort)
	}

	if nodePort, ok := util.CoalesceEnv("NODEPORT", ""); ok && nodePort == "true" {
		podHostname, _ := os.Hostname()
		announcePortVar := "announce_port_" + strings.ReplaceAll(podHostname, "-", "_")
		announceBusPortVar := "announce_bus_port_" + strings.ReplaceAll(podHostname, "-", "_")

		// Get environment variables
		clusterAnnouncePort := os.Getenv(announcePortVar)
		clusterAnnounceBusPort := os.Getenv(announceBusPortVar)

		if clusterAnnouncePort != "" {
			cfg.Append("cluster-announce-port", clusterAnnouncePort)
		}
		if clusterAnnounceBusPort != "" {
			cfg.Append("cluster-announce-bus-port", clusterAnnounceBusPort)
		}
	}

	if _, err := os.Stat(externalConfigFile); err == nil {
		cfg.Append("include", externalConfigFile)
	}

	// Add cluster announcement IP and hostname for cluster mode
	if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
		var err error
		var clusterAnnounceIP string
		if nodePort, ok := util.CoalesceEnv("NODEPORT", ""); ok && nodePort == "true" {
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
	}

	if maxMemory, ok := util.CoalesceEnv(consts.ENV_KEY_REDIS_MAX_MEMORY, ""); ok && maxMemory != "" {
		cfg.Append("maxmemory", maxMemory)
	}

	return cfg.Commit()
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
