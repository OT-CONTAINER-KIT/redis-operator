package consts

const (
	ENV_KEY_REDIS_MAX_MEMORY = "REDIS_MAX_MEMORY"
	// ENV_KEY_EXPAND_EXTERNAL_CONFIG gates env-var expansion of the user's
	// external (ConfigMap-mounted) redis config before it is included. Off by
	// default to preserve historical behaviour; set to "true" to expand
	// ${VAR}/$VAR references (e.g. `requirepass ${REDIS_PASSWORD}`) against the
	// container environment so they are not loaded as literal strings.
	ENV_KEY_EXPAND_EXTERNAL_CONFIG = "EXPAND_EXTERNAL_CONFIG"
)
