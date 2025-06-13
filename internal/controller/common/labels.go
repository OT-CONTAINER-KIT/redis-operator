package common

type SetupType string

const (
	SetupTypeStandalone  SetupType = "standalone"
	SetupTypeReplication SetupType = "replication"
	SetupTypeCluster     SetupType = "cluster"
	SetupTypeSentinel    SetupType = "sentinel"
)

func GetRedisLabels(name string, st SetupType, role string, labels map[string]string) map[string]string {
	lbls := getRedisStableLabels(name, string(st), role)
	for k, v := range labels {
		lbls[k] = v
	}
	return lbls
}

func getRedisStableLabels(name, st, role string) map[string]string {
	return map[string]string{
		"app":              name,
		"redis_setup_type": st,
		"role":             role,
	}
}
