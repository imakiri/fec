package fec

import (
	"testing"
)

func TestFEC(t *testing.T) {
	//var ctx = context.Background()
	//var rs, err = NewCodec()
	//assert.NoError(t, err)
	//
	//var enc = rs.Encoder(new(uint64), new(uint64))
	//var dec = rs.Decoder(new(uint64), new(uint64))
	//
	//const size = 100 * 11680
	//var data = make([]byte, size)
	//n, _ := rand.Read(data)
	//assert.Equal(t, len(data), n)
	//var senderBuf = bytes.NewBuffer(data)
	//var receiverBuf = bytes.NewBuffer(make([]byte, 0, size))
	//
	//Encode(ctx, io.NopCloser(senderBuf), io.NopCloser(), 8, 8)
	//
	//var ctx = context.Background()
	//var buf = bytes.NewBuffer(nil)
	//
	//err = enc.Encode(ctx, senderBuf, buf)
	//assert.True(t, errors.Is(err, io.EOF))
	//t.Log("buf len ", buf.Len())
	//
	//err = dec.Decode(ctx, buf, receiverBuf)
	//t.Log("receiver buf len ", receiverBuf.Len())
	//
	//assert.True(t, errors.Is(err, io.EOF))
	//
	//for i := range data {
	//	assert.Equal(t, data[i], receiverBuf.Bytes()[i])
	//}
}
