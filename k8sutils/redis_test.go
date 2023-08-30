// checkRedisNodePresence
package k8sutils

import (
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
)

func TestCheckRedisNodePresence(t *testing.T) {
	cr := &redisv1beta2.RedisCluster{}
	output := "205dd1780dda981f9320c9d47d069b3c0ceaa358 172.17.0.24:6379@16379 slave b65312dcf5537b8826c344783f078096fdb7f27c 0 1654197347000 1 connected\nfaa21623054227826e93dd71314cce3706491dac 172.17.0.28:6379@16379 slave d54557b21bc5a5aa947ce58b7dbadc5d39bdd551 0 1654197347000 2 connected\nb65312dcf5537b8826c344783f078096fdb7f27c 172.17.0.25:6379@16379 master - 0 1654197346000 1 connected 0-5460\nd54557b21bc5a5aa947ce58b7dbadc5d39bdd551 172.17.0.29:6379@16379 myself,master - 0 1654197347000 2 connected 5461-10922\nc9fa05269c4e662295bf34eb93f1315f962493ba 172.17.0.3:6379@16379 master - 0 1654197348006 3 connected 10923-16383"
	csvOutput := csv.NewReader(strings.NewReader(output))
	csvOutput.Comma = ' '
	csvOutput.FieldsPerRecord = -1
	nodes, _ := csvOutput.ReadAll()

	var tests = []struct {
		nodes [][]string
		ip    string
		want  bool
	}{
		{nodes, "172.17.0.24", true},
		{nodes, "172.17.0.111", false},
		{nodes, "172.17.0.2", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s,%s", tt.nodes, tt.ip)
		t.Run(testname, func(t *testing.T) {
			ans := checkRedisNodePresence(cr, tt.nodes, tt.ip)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}
