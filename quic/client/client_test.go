package client

import "testing"

func TestName(t *testing.T) {
	QUICClientRun("udp://127.0.0.1:8765")
}
