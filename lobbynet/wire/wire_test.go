package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

func TestRoundTrip(t *testing.T) {
	var b bytes.Buffer
	in := []*lobbyv1.Envelope{
		{Id: 1, Msg: &lobbyv1.Envelope_Hello{Hello: &lobbyv1.Hello{Protocol: 3, Build: "x"}}},
		{Msg: &lobbyv1.Envelope_Frame{Frame: &lobbyv1.GameFrame{From: 1, To: 0x7ffe, Msg: 0x12,
			Payload: bytes.Repeat([]byte{0xab}, 0xffff)}}},
		{Msg: &lobbyv1.Envelope_Ping{Ping: &lobbyv1.Ping{}}},
	}
	for _, env := range in {
		if err := Write(&b, env); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range in {
		got, err := Read(&b)
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(got, want) {
			t.Fatalf("envelope %d: got %v", i, got)
		}
	}
	if _, err := Read(&b); err != io.EOF {
		t.Fatalf("at the end: %v, want EOF", err)
	}
}

func TestBadFrames(t *testing.T) {
	var big [4]byte
	binary.BigEndian.PutUint32(big[:], MaxFrame+1)
	if _, err := Read(bytes.NewReader(big[:])); err == nil {
		t.Error("read an oversized frame")
	}
	short := []byte{0, 0, 0, 5, 1, 2}
	if _, err := Read(bytes.NewReader(short)); err != io.ErrUnexpectedEOF {
		t.Errorf("cut frame: %v, want ErrUnexpectedEOF", err)
	}
	junk := []byte{0, 0, 0, 2, 0xff, 0xff}
	if _, err := Read(bytes.NewReader(junk)); err == nil {
		t.Error("read a frame that is not an envelope")
	}
}
