package colong

import (
	"fmt"
	"testing"
)

func TestSharePub(t *testing.T) {
	sp := NewSharePubMessage()
	sp.SetPacketId(12)
	sp.SetPayload([]byte("你好"))
	sp.SetTopic([]byte("$share/lalal/ss"))
	sp.SetQoS(0x01)
	fmt.Println(sp)
	b := make([]byte, sp.Len())
	i, _ := sp.Encode(b)
	newSp := NewSharePubMessage()
	newSp.Decode(b[:i])
	fmt.Println(newSp)
	fmt.Println(newSp.ShareName())
}
