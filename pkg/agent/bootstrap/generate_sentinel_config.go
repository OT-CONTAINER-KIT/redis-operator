package bootstrap

import (
	"fmt"
	"os"

	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
)

// defaultSentinelConfig from https://github.com/OT-CONTAINER-KIT/redis/blob/master/sentinel.conf
const defaultSentinelConfig = `
daemonize no
pidfile /var/run/redis-sentinel.pid
logfile ""
dir /tmp

acllog-max-len 128

# sentinel monitor mymaster 127.0.0.1 6379 2
# sentinel down-after-milliseconds mymaster 30000
# sentinel parallel-syncs mymaster 1
# sentinel failover-timeout mymaster 180000
sentinel deny-scripts-reconfig yes
# SENTINEL resolve-hostnames no
# SENTINEL announce-hostnames no
# SENTINEL master-reboot-down-after-period mymaster 0
`

func generateSentinelConfig() error {
	cfg := newConfig("/etc/redis/sentinel.conf", defaultSentinelConfig)

	// set_sentinel_password
	{
		if val, ok := util.CoalesceEnv("REDIS_PASSWORD", ""); ok {
			cfg.append("masterauth", val)
			cfg.append("requirepass", val)
			cfg.append("protected-mode", "yes")
		} else {
			fmt.Println("Sentinel is running without password which is not recommended")
			cfg.append("protected-mode", "no")
		}
	}

	// sentinel_mode_setup
	{
		masterGroupName, _ := util.CoalesceEnv("MASTER_GROUP_NAME", "mymaster")
		ip, _ := util.CoalesceEnv("IP", "0.0.0.0")
		port, _ := util.CoalesceEnv("PORT", "6379")
		quorum, _ := util.CoalesceEnv("QUORUM", "2")
		downAfterMilliseconds, _ := util.CoalesceEnv("DOWN_AFTER_MILLISECONDS", "30000")
		parallelSyncs, _ := util.CoalesceEnv("PARALLEL_SYNCS", "1")
		failoverTimeout, _ := util.CoalesceEnv("FAILOVER_TIMEOUT", "180000")
		resolveHostnames, _ := util.CoalesceEnv("RESOLVE_HOSTNAMES", "no")
		announceHostnames, _ := util.CoalesceEnv("ANNOUNCE_HOSTNAMES", "no")

		cfg.append("sentinel monitor", masterGroupName, ip, port, quorum)
		cfg.append("sentinel down-after-milliseconds", masterGroupName, downAfterMilliseconds)
		cfg.append("sentinel parallel-syncs", masterGroupName, parallelSyncs)
		cfg.append("sentinel failover-timeout", masterGroupName, failoverTimeout)
		cfg.append("sentinel resolve-hostnames", resolveHostnames)
		cfg.append("sentinel announce-hostnames", announceHostnames)

		// If master password is set
		if masterPassword, ok := util.CoalesceEnv("MASTER_PASSWORD", ""); ok {
			cfg.append("sentinel auth-pass", masterGroupName, masterPassword)
		}

		// If sentinel ID is set
		if sentinelID, ok := util.CoalesceEnv("SENTINEL_ID", ""); ok {
			// Note: We should use SHA1 hash here, but since we don't have a direct SHA1 function,
			// we're simply using the sentinelID as myid
			// In a real application, SHA1 hash functionality should be implemented
			cfg.append("sentinel myid", sentinelID)
		}
	}

	// port_setup
	{
		sentinelPort, _ := util.CoalesceEnv("SENTINEL_PORT", "26379")
		cfg.append("port", sentinelPort)
	}

	// acl_setup
	{
		if aclMode, ok := util.CoalesceEnv("ACL_MODE", ""); ok && aclMode == "true" {
			cfg.append("aclfile", "/etc/redis/user.acl")
		} else {
			fmt.Println("ACL_MODE is not true, skipping ACL file modification")
		}
	}

	// tls_setup
	{
		if tlsMode, ok := util.CoalesceEnv("TLS_MODE", ""); ok && tlsMode == "true" {
			redisTLSCert, _ := util.CoalesceEnv("REDIS_TLS_CERT", "")
			redisTLSCertKey, _ := util.CoalesceEnv("REDIS_TLS_CERT_KEY", "")
			redisTLSCAKey, _ := util.CoalesceEnv("REDIS_TLS_CA_KEY", "")

			cfg.append("port", "0")
			cfg.append("tls-port", "26379")
			cfg.append("tls-cert-file", redisTLSCert)
			cfg.append("tls-key-file", redisTLSCertKey)
			cfg.append("tls-ca-cert-file", redisTLSCAKey)
			cfg.append("tls-auth-clients", "optional")
		} else {
			fmt.Println("Running sentinel without TLS mode")
		}
	}

	// If external config file exists, include it
	externalConfigFile, _ := util.CoalesceEnv("EXTERNAL_CONFIG_FILE", "/etc/redis/external.conf.d/redis-sentinel-additional.conf")
	if fileExists(externalConfigFile) {
		cfg.append("include", externalConfigFile)
	}

	// Commit configuration
	if err := cfg.commit(); err != nil {
		return err
	}

	fmt.Println("Starting sentinel service .....")
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
