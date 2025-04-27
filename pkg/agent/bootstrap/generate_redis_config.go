package bootstrap

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
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

// GenerateRedisConfig generates Redis configuration file
func generateRedisConfig() error {
	cfg := newConfig("/etc/redis/redis.conf", defaultRedisConfig)

	var (
		persistenceEnabled, _ = util.CoalesceEnv("PERSISTENCE_ENABLED", "false")
		dataDir, _            = util.CoalesceEnv("DATA_DIR", "/data")
		nodeConfDir, _        = util.CoalesceEnv("NODE_CONF_DIR", "/node-conf")
		externalConfigFile, _ = util.CoalesceEnv("EXTERNAL_CONFIG_FILE", "/etc/redis/external.conf.d/redis-additional.conf")
		redisMajorVersion, _  = util.CoalesceEnv("REDIS_MAJOR_VERSION", "v7")
	)

	// set_redis_password - configure Redis password
	{
		if val, ok := util.CoalesceEnv("REDIS_PASSWORD", ""); ok && val != "" {
			cfg.append("masterauth", val)
			cfg.append("requirepass", val)
			cfg.append("protected-mode", "yes")
		} else {
			fmt.Println("Redis is running without password which is not recommended")
			cfg.append("protected-mode", "no")
		}
	}

	// redis_mode_setup - configure Redis mode (cluster or standalone)
	{
		if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
			cfg.append("cluster-enabled", "yes")
			cfg.append("cluster-node-timeout", "5000")
			cfg.append("cluster-require-full-coverage", "no")
			cfg.append("cluster-migration-barrier", "1")
			cfg.append("cluster-config-file", fmt.Sprintf("%s/nodes.conf", nodeConfDir))

			// Get Pod IP
			cmd := exec.Command("hostname", "-i")
			output, err := cmd.Output()
			if err != nil {
				log.Printf("Warning: Failed to get pod IP: %v", err)
			} else {
				podIP := strings.TrimSpace(string(output))

				// Update IP in nodes.conf file
				nodesConfPath := filepath.Join(nodeConfDir, "nodes.conf")
				if _, err := os.Stat(nodesConfPath); err == nil {
					cmd := exec.Command("sed", "-i", fmt.Sprintf("/myself/ s/[0-9]\\{1,3\\}\\.[0-9]\\{1,3\\}\\.[0-9]\\{1,3\\}\\.[0-9]\\{1,3\\}/%s/", podIP), nodesConfPath)
					if err := cmd.Run(); err != nil {
						log.Printf("Warning: Failed to update nodes.conf: %v", err)
					}
				}
			}
		} else {
			fmt.Println("Setting up redis in standalone mode")
		}
	}

	// tls_setup - configure TLS
	{
		if tlsMode, ok := util.CoalesceEnv("TLS_MODE", ""); ok && tlsMode == "true" {
			redisTLSCert, _ := util.CoalesceEnv("REDIS_TLS_CERT", "")
			redisTLSCertKey, _ := util.CoalesceEnv("REDIS_TLS_CERT_KEY", "")
			redisTLSCAKey, _ := util.CoalesceEnv("REDIS_TLS_CA_KEY", "")

			cfg.append("tls-cert-file", redisTLSCert)
			cfg.append("tls-key-file", redisTLSCertKey)
			cfg.append("tls-ca-cert-file", redisTLSCAKey)
			cfg.append("tls-auth-clients", "optional")
			cfg.append("tls-replication", "yes")

			if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
				cfg.append("tls-cluster", "yes")

				if redisMajorVersion == "v7" {
					cfg.append("cluster-preferred-endpoint-type", "hostname")
				}
			}
		} else {
			fmt.Println("Running without TLS mode")
		}
	}

	// acl_setup - configure ACL
	{
		if aclMode, ok := util.CoalesceEnv("ACL_MODE", ""); ok && aclMode == "true" {
			cfg.append("aclfile", "/etc/redis/user.acl")
		} else {
			fmt.Println("ACL_MODE is not true, skipping ACL file modification")
		}
	}

	// persistence_setup - configure persistence
	{
		if persistenceEnabled == "true" {
			cfg.append("save", "900 1")
			cfg.append("save", "300 10")
			cfg.append("save", "60 10000")
			cfg.append("appendonly", "yes")
			cfg.append("appendfilename", "\"appendonly.aof\"")
			cfg.append("dir", dataDir)
		} else {
			fmt.Println("Running without persistence mode")
		}
	}

	// port_setup - configure ports
	{
		redisPort, _ := util.CoalesceEnv("REDIS_PORT", "6379")

		if tlsMode, ok := util.CoalesceEnv("TLS_MODE", ""); ok && tlsMode == "true" {
			cfg.append("port", "0")
			cfg.append("tls-port", redisPort)
		} else {
			cfg.append("port", redisPort)
		}

		if nodePort, ok := util.CoalesceEnv("NODEPORT", ""); ok && nodePort == "true" {
			podHostname, _ := os.Hostname()
			announcePortVar := "announce_port_" + strings.ReplaceAll(podHostname, "-", "_")
			announceBusPortVar := "announce_bus_port_" + strings.ReplaceAll(podHostname, "-", "_")

			// Get environment variables
			clusterAnnouncePort := os.Getenv(announcePortVar)
			clusterAnnounceBusPort := os.Getenv(announceBusPortVar)

			if clusterAnnouncePort != "" {
				cfg.append("cluster-announce-port", clusterAnnouncePort)
			}
			if clusterAnnounceBusPort != "" {
				cfg.append("cluster-announce-bus-port", clusterAnnounceBusPort)
			}
		}
	}

	// external_config - include external config file
	{
		if _, err := os.Stat(externalConfigFile); err == nil {
			cfg.append("include", externalConfigFile)
		}
	}

	// Add cluster announcement IP and hostname for cluster mode
	if setupMode, ok := util.CoalesceEnv("SETUP_MODE", ""); ok && setupMode == "cluster" {
		// Get Pod hostname and IP
		podHostname, err := os.Hostname()
		if err == nil {
			var clusterAnnounceIP string

			if nodePort, ok := util.CoalesceEnv("NODEPORT", ""); ok && nodePort == "true" {
				clusterAnnounceIP = os.Getenv("HOST_IP")
			} else {
				cmd := exec.Command("hostname", "-i")
				output, err := cmd.Output()
				if err == nil {
					clusterAnnounceIP = strings.TrimSpace(string(output))
				}
			}
			if clusterAnnounceIP != "" {
				cfg.append("cluster-announce-ip", clusterAnnounceIP)
			}
			if redisMajorVersion == "v7" {
				cfg.append("cluster-announce-hostname", podHostname)
			}
		}
	}

	// Commit configuration to file
	return cfg.commit()
}
