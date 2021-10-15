package didexchange

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/findy-network/findy-agent/agent/sec"
	"github.com/findy-network/findy-agent/agent/ssi"
	"github.com/findy-network/findy-wrapper-go/dto"
	"github.com/golang/glog"
	"github.com/lainio/err2"
)

const connectionSigExpTime = 10 * 60 * 60

func (connection *Connection) buildConnectionSignature(pipe sec.Pipe) (cs *ConnectionSignature, err error) {
	defer err2.Annotate("build connection sign", &err)

	connectionJSON := err2.Bytes.Try(json.Marshal(connection))

	signedData, signature, verKey := pipe.SignAndStamp(connectionJSON)

	return &ConnectionSignature{
		Type:       "did:sov:BzCbsNYhMrjHiqZDTUASHg;spec/signature/1.0/ed25519Sha512_single",
		SignedData: base64.URLEncoding.EncodeToString(signedData),
		SignVerKey: verKey,
		Signature:  base64.URLEncoding.EncodeToString(signature),
	}, nil
}

// verifySignature verifies a signature inside the structure. If sec.Pipe is not
// given, it uses the key from the signature structure. If succeeded it returns
// a Connection structure, else nil.
func (cs *ConnectionSignature) verifySignature(pipe *sec.Pipe) (c *Connection, err error) {
	defer err2.Annotate("verify sign", &err)

	if pipe != nil && pipe.Out.VerKey() != cs.SignVerKey {
		s := "programming error, we shouldn't be here"
		glog.Error(s)
		panic(s)
	} else if pipe == nil { // we need a tmp DID for a tmp Pipe
		did := ssi.NewDid("", cs.SignVerKey)
		pipe = &sec.Pipe{Out: did}
	}

	data := err2.Bytes.Try(decodeB64(cs.SignedData))
	if len(data) == 0 {
		s := "missing or invalid signature data"
		glog.Error(s)
		return nil, fmt.Errorf(s)
	}

	signature := err2.Bytes.Try(decodeB64(cs.Signature))

	ok, _ := pipe.Verify(data, signature)
	if !ok {
		glog.Error("cannot verify signature")
		return nil, nil
	}

	timestamp := int64(binary.BigEndian.Uint64(data))

	if time.Now().Unix()-timestamp > connectionSigExpTime {
		glog.Error("timestamp too old, connection verify signature!")
		return nil, nil
	}

	glog.V(3).Info("verified connection signature w/ ts:", time.Unix(timestamp, 0))

	connectionJSON := data[8:]

	var connection Connection
	dto.FromJSON(connectionJSON, &connection)

	return &connection, nil
}

func decodeB64(str string) ([]byte, error) {
	data, err := base64.URLEncoding.DecodeString(str)
	if err != nil {
		data, err = base64.RawURLEncoding.DecodeString(str)
	}
	return data, err
}
