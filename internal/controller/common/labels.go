package common

type SetupType string

const (
	SetupTypeStandalone  SetupType = "standalone"
	SetupTypeReplication SetupType = "replication"
	SetupTypeCluster     SetupType = "cluster"
	SetupTypeSentinel    SetupType = "sentinel"
)

func GetRedisLabels(name string, st SetupType, role string, labels map[string]string) map[string]string {
	return GetRedisLabelsWithAdditional(name, st, role, labels, nil)
}

func GetRedisLabelsWithAdditional(name string, st SetupType, role string, labels, additionalLabels map[string]string) map[string]string {
	lbls := map[string]string{}
	for k, v := range labels {
		lbls[k] = v
	}
	for k, v := range additionalLabels {
		lbls[k] = v
	}
	for k, v := range getRedisStableLabels(name, string(st), role) {
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
