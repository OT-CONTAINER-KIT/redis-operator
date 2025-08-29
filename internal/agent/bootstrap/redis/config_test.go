package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func Test_updateMyselfIP(t *testing.T) {
	testData := `7a6b5f4f99496c97f4e32c30c077aa95cab92664 10.244.0.246:0@16379,,tls-port=6379,shard-id=a03445a0d3f6d405af261041e0cb77a8a176f42b slave b66f2fa597eeda567cf05f3701419be9a3b2f50e 0 1756463509000 1 connected
93ad60e9ce21430683a3534d2c96ab1b8077cfe8 10.244.0.237:0@16379,,tls-port=6379,shard-id=2f177491b895051f91e91e554a2a9da2cd167aeb master - 0 1756463509685 2 connected 5461-10922
b66f2fa597eeda567cf05f3701419be9a3b2f50e 10.244.0.222:0@16379,,tls-port=6379,shard-id=a03445a0d3f6d405af261041e0cb77a8a176f42b myself,master - 0 0 1 connected 0-5460
88456cac1830f3e00f6ab681fb819b4b1d7ad36b 10.244.0.252:0@16379,,tls-port=6379,shard-id=b61110535f09b9c0703517f79da79118fee8d1a4 slave 580e234a8dcd74717c37d01ed8097929c64536ff 0 1756463509691 3 connected
580e234a8dcd74717c37d01ed8097929c64536ff 10.244.0.240:0@16379,,tls-port=6379,shard-id=b61110535f09b9c0703517f79da79118fee8d1a4 master - 0 1756463509583 3 connected 10923-16383
c0fc3c21460fec045775d2dcde220fb26ca668c1 10.244.0.249:0@16379,,tls-port=6379,shard-id=2f177491b895051f91e91e554a2a9da2cd167aeb slave 93ad60e9ce21430683a3534d2c96ab1b8077cfe8 0 1756463509583 2 connected
vars currentEpoch 3 lastVoteEpoch 0
`

	tmpFile := "/tmp/test_nodes.conf"
	err := os.WriteFile(tmpFile, []byte(testData), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	newIP := "10.244.0.9"
	updated, err := updateMyselfIP(tmpFile, newIP)
	if err != nil {
		t.Errorf("updateMyselfIP() error = %v", err)
	}

	if updated == nil {
		t.Errorf("Expected changes to be made, but updated is nil")
		return
	}
	expectedContent := strings.ReplaceAll(testData, "10.244.0.222", newIP)
	updatedContent := string(updated)

	if updatedContent != expectedContent {
		t.Errorf("Expected updated content to match:\nExpected:\n%s\nGot:\n%s", expectedContent, updatedContent)
	}

	t.Logf("Successfully updated nodes.conf with new IP %s", newIP)
}
