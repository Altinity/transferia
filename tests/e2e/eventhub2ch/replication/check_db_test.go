package replication

import "testing"

func TestReplicationSmoke(t *testing.T) {
	t.Skip("blocked: eventhub2ch local smoke is not wired yet; requires stable EventHub emulator + end-to-end recipe. See ../README.md")
}
