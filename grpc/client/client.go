package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/findy-network/findy-agent-api/grpc/agency"
	didexchange "github.com/findy-network/findy-agent/std/didexchange/invitation"
	"github.com/findy-network/findy-grpc/jwt"
	"github.com/findy-network/findy-grpc/rpc"
	"github.com/findy-network/findy-wrapper-go/dto"
	"github.com/golang/glog"
	"github.com/lainio/err2"
	"google.golang.org/grpc"
)

var Conn *grpc.ClientConn

type Pairwise struct {
	ID    string
	Label string
}

func OkStatus(s *agency.ProtocolState) bool {
	return s.State == agency.ProtocolState_OK
}

func TryOpenConn(user, addr string, port int) *grpc.ClientConn {
	conn, err := OpenClientConn(user, fmt.Sprintf("%s:%d", addr, port))
	err2.Check(err)
	return conn
}

func OpenClientConn(user, addr string) (conn *grpc.ClientConn, err error) {
	defer err2.Return(&err)

	if Conn != nil {
		return nil, errors.New("client connection all ready open")
	}
	pki := rpc.LoadPKI()
	glog.V(5).Infoln("client with user:", user)
	conn, err = rpc.ClientConn(rpc.ClientCfg{
		PKI:  *pki,
		JWT:  jwt.BuildJWT(user),
		Addr: addr,
		TLS:  true,
	})
	err2.Check(err)
	Conn = conn
	return
}

func (pw Pairwise) Issue(ctx context.Context, credDefID, attrsJSON string) (ch chan *agency.ProtocolState, err error) {
	protocol := &agency.Protocol{
		ConnectionId: pw.ID,
		TypeId:       agency.Protocol_ISSUE,
		Role:         agency.Protocol_INITIATOR,
		StartMsg: &agency.Protocol_CredDef{CredDef: &agency.Protocol_Issuing{
			CredDefId:      credDefID,
			AttributesJson: attrsJSON,
		}},
	}
	return doStart(ctx, protocol)
}

func Connection(ctx context.Context, invitationJSON string) (connID string, ch chan *agency.ProtocolState, err error) {
	defer err2.Return(&err)

	// assert that invitation is OK, and we need to return the connection ID
	// because it's the task id as well
	var invitation didexchange.Invitation
	dto.FromJSONStr(invitationJSON, &invitation)

	protocol := &agency.Protocol{
		TypeId:   agency.Protocol_CONNECT,
		Role:     agency.Protocol_INITIATOR,
		StartMsg: &agency.Protocol_InvitationJson{InvitationJson: invitationJSON},
	}
	ch, err = doStart(ctx, protocol)
	err2.Check(err)
	connID = invitation.ID
	return connID, ch, err
}

func (pw Pairwise) Ping(ctx context.Context) (ch chan *agency.ProtocolState, err error) {
	protocol := &agency.Protocol{
		ConnectionId: pw.ID,
		TypeId:       agency.Protocol_TRUST_PING,
		Role:         agency.Protocol_INITIATOR,
	}
	return doStart(ctx, protocol)
}

func (pw Pairwise) ReqProof(ctx context.Context, proofAttrs string) (ch chan *agency.ProtocolState, err error) {
	protocol := &agency.Protocol{
		ConnectionId: pw.ID,
		TypeId:       agency.Protocol_PROOF,
		Role:         agency.Protocol_INITIATOR,
		StartMsg:     &agency.Protocol_ProofAttributesJson{ProofAttributesJson: proofAttrs},
	}
	return doStart(ctx, protocol)
}

func Listen(ctx context.Context, protocol *agency.ClientID) (ch chan *agency.AgentStatus, err error) {
	defer err2.Return(&err)

	c := agency.NewAgentClient(Conn)
	statusCh := make(chan *agency.AgentStatus)

	stream, err := c.Listen(ctx, protocol)
	err2.Check(err)
	glog.V(0).Infoln("successful start of listen id:", protocol.Id)
	go func() {
		defer err2.CatchTrace(func(err error) {
			glog.Warningln("error when reading response:", err)
			close(statusCh)
		})
		for {
			status, err := stream.Recv()
			if err == io.EOF {
				glog.V(3).Infoln("status stream end")
				close(statusCh)
				break
			}
			err2.Check(err)
			statusCh <- status
		}
	}()
	return statusCh, nil
}

func doStart(ctx context.Context, protocol *agency.Protocol) (ch chan *agency.ProtocolState, err error) {
	defer err2.Return(&err)

	c := agency.NewDIDCommClient(Conn)
	statusCh := make(chan *agency.ProtocolState)

	stream, err := c.Run(ctx, protocol)
	err2.Check(err)
	glog.V(3).Infoln("successful start of:", protocol.TypeId)
	go func() {
		defer err2.CatchTrace(func(err error) {
			glog.V(3).Infoln("err when reading response", err)
			close(statusCh)
		})
		for {
			status, err := stream.Recv()
			if err == io.EOF {
				glog.V(3).Infoln("status stream end")
				close(statusCh)
				break
			}
			err2.Check(err)
			statusCh <- status
		}
	}()
	return statusCh, nil
}
