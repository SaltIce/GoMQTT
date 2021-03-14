package quic

import "testing"

func TestQuicServer(t *testing.T) {
	QUIC_Server_Run()
}

func TestQuicClient(t *testing.T) {
	QUIC_Client_run()
}
