package onboard

import (
	"errors"
	"io"

	"github.com/findy-network/findy-agent/agent/agency"
	"github.com/findy-network/findy-agent/agent/cloud"
	"github.com/findy-network/findy-agent/agent/endp"
	"github.com/findy-network/findy-agent/agent/handshake"
	"github.com/findy-network/findy-agent/agent/mesg"
	"github.com/findy-network/findy-agent/agent/pltype"
	"github.com/findy-network/findy-agent/agent/service"
	"github.com/findy-network/findy-agent/agent/ssi"
	"github.com/findy-network/findy-agent/agent/utils"
	"github.com/findy-network/findy-agent/cmds"
	didexchange "github.com/findy-network/findy-agent/std/didexchange/invitation"
	"github.com/findy-network/findy-wrapper-go/dto"
	"github.com/golang/glog"
	"github.com/lainio/err2"
	"github.com/lainio/err2/assert"
)

type Cmd struct {
	cmds.Cmd
	Email      string
	AgencyAddr string
}

type Result struct {
	CADID       string
	ServiceAddr service.Addr
	didexchange.Invitation
}

func (c Cmd) Validate() error {
	if err := c.Cmd.Validate(); err != nil {
		return err
	}
	if err := c.Cmd.ValidateWalletExistence(false); err != nil {
		return err
	}
	if c.Email == "" {
		return errors.New("email cannot be empty")
	}
	if c.AgencyAddr == "" {
		return errors.New("agency url cannot be empty")
	}
	return nil
}

func (c Cmd) InitWebExec() *cloud.Agent {
	defer err2.CatchTrace(func(err error) {
		glog.Error(err)
	})

	edge := struct {
		*ssi.Wallet
		*cloud.Agent
	}{
		Wallet: ssi.NewRawWalletCfg(c.WalletName, c.WalletKey),
		Agent:  cloud.NewEA(),
	}
	aw := edge.Wallet

	aw.Create()
	edge.Agent.OpenWallet(*aw)
	glog.V(1).Info("init web edge ok")
	return edge.Agent
}

// WebExec is identical with Exec BUT the edgeAgent is given to it because the
// web wallets use the same web wallet for all of them. Only pairwise is
// actually used at the moment.
func (c Cmd) WebExec(edgeAgent *cloud.Agent, progress io.Writer) (r Result, err error) {
	assert.D.True(edgeAgent != nil)

	defer err2.Annotate("web wallet on-boarding", &err)

	cmds.Fprintln(progress, "Web wallet handshake starting...")

	msg := mesg.NewHandshake(c.Email, pltype.HandshakePairwiseName)

	endpointAdd := &endp.Addr{
		BasePath: c.AgencyAddr,
		Service:  agency.APIPath,
		PlRcvr:   handshake.HandlerEndpoint,
	}

	cmds.Fprintln(progress, "Requesting server.")

	glog.V(1).Infoln("send handshake", endpointAdd)
	payload, e := cmds.SendHandshake(msg, endpointAdd)
	err2.Check(e)

	cmds.Fprintln(progress, "\nBuilding result to server...")
	responsePayload, _ := edgeAgent.ProcessPL(mesg.NewPayloadImpl(payload))

	ns := responsePayload.ID()
	nonce := utils.ParseNonce(ns)

	// In all cases we must build the endpoint, server cannot give it us int
	// phase of the handshake.
	endpointAddress := &endp.Addr{
		BasePath: c.AgencyAddr, // the base address of the receiving server
		Service:  agency.APIPath,
		PlRcvr:   handshake.HandlerEndpoint, // we keep the message type in payload same during the whole sequence
		MsgRcvr:  payload.Message.Did,       // The inner message is ENCRYPTED according the payload's DID
	}

	// 3. we send our results and get CONN_ACK back
	cmds.Fprintln(progress, "Sending handshake results to server.")
	finalPl, err := cmds.SendAndWaitDIDComPayload(responsePayload, endpointAddress, nonce)
	err2.Check(err)

	// get our connection/invite endpoint to print and return it in eaEnp
	respMesg := finalPl.Message().FieldObj().(*mesg.Msg)
	invit := didexchange.Invitation{
		ID:              utils.UUID(),
		Type:            pltype.AriesConnectionInvitation,
		ServiceEndpoint: respMesg.RcvrEndp,
		RecipientKeys:   []string{respMesg.RcvrKey},
		Label:           c.Email,
	}
	cmds.Fprintln(progress, "----- invitation JSON begin -----")
	invitJSON := dto.ToJSON(&invit)
	cmds.Fprintln(progress, invitJSON)
	cmds.Fprintln(progress, "----- invitation JSON end -----")

	eaEnp := service.Addr{
		Endp: respMesg.RcvrEndp,
		Key:  respMesg.RcvrKey,
	}

	// When connection request is started by other end they reset the nonce
	if finalPl.Type() == pltype.ConnectionAck {
		n := utils.NonceToStr(nonce)
		if n != finalPl.Message().Nonce() && finalPl.ID() != ns {
			cmds.Fprintln(progress, "CA send ERROR, nonce mismatch")
		}
	}

	// let's process this as well, so that endpoint will be stored, etc.
	responsePayload, _ = edgeAgent.ProcessPL(finalPl)

	return Result{
		CADID:       edgeAgent.Pw().YouDID(),
		ServiceAddr: eaEnp,
		Invitation:  invit,
	}, nil
}

func (c Cmd) Exec(progress io.Writer) (r Result, err error) {
	defer err2.Annotate("on-boarding", &err)

	cmds.Fprintln(progress, "Handshake starting...")

	edge := struct {
		*ssi.Wallet
		*cloud.Agent
	}{
		Wallet: ssi.NewRawWalletCfg(c.WalletName, c.WalletKey),
		Agent:  cloud.NewEA(),
	}
	aw := edge.Wallet
	createStarted := make(chan struct{})
	go func() {
		defer err2.CatchTrace(func(err error) {
			glog.Error(err)
			createStarted <- struct{}{}
		})
		aw.Create()
		edge.Agent.OpenWallet(*aw)
		createStarted <- struct{}{}
	}()
	defer edge.Agent.CloseWallet()

	msg := mesg.NewHandshake(c.Email, pltype.HandshakePairwiseName)

	endpointAdd := &endp.Addr{
		BasePath: c.AgencyAddr,
		Service:  agency.APIPath,
		PlRcvr:   handshake.HandlerEndpoint,
	}

	cmds.Fprintln(progress, "Requesting server.")
	done := cmds.Progress(progress)

	payload, e := cmds.SendHandshake(msg, endpointAdd)
	done <- struct{}{}
	err2.Check(e)

	<-createStarted // safe to process with the wallet
	cmds.Fprintln(progress, "\nBuilding result to server...")
	responsePayload, _ := edge.Agent.ProcessPL(mesg.NewPayloadImpl(payload))

	ns := responsePayload.ID()
	nonce := utils.ParseNonce(ns)

	// In all cases we must build the endpoint, server cannot give it us int
	// phase of the handshake.
	endpointAddress := &endp.Addr{
		BasePath: c.AgencyAddr, // the base address of the receiving server
		Service:  agency.APIPath,
		PlRcvr:   handshake.HandlerEndpoint, // we keep the message type in payload same during the whole sequence
		MsgRcvr:  payload.Message.Did,       // The inner message is ENCRYPTED according the payload's DID
	}

	// 3. we send our results and get CONN_ACK back
	cmds.Fprintln(progress, "Sending handshake results to server.")
	done = cmds.Progress(progress)
	finalPl, err := cmds.SendAndWaitDIDComPayload(responsePayload, endpointAddress, nonce)
	done <- struct{}{}
	err2.Check(err)

	// get our connection/invite endpoint to print and return it in eaEnp
	respMesg := finalPl.Message().FieldObj().(*mesg.Msg)
	invit := didexchange.Invitation{
		ID:              utils.UUID(),
		Type:            pltype.AriesConnectionInvitation,
		ServiceEndpoint: respMesg.RcvrEndp,
		RecipientKeys:   []string{respMesg.RcvrKey},
		Label:           c.Email,
	}
	cmds.Fprintln(progress, "----- invitation JSON begin -----")
	invitJSON := dto.ToJSON(&invit)
	cmds.Fprintln(progress, invitJSON)
	cmds.Fprintln(progress, "----- invitation JSON end -----")

	eaEnp := service.Addr{
		Endp: respMesg.RcvrEndp,
		Key:  respMesg.RcvrKey,
	}

	// When connection request is started by other end they reset the nonce
	if finalPl.Type() == pltype.ConnectionAck {
		n := utils.NonceToStr(nonce)
		if n != finalPl.Message().Nonce() && finalPl.ID() != ns {
			cmds.Fprintln(progress, "CA send ERROR, nonce mismatch")
		}
	}

	// let's process this as well, so that endpoint will be stored, etc.
	responsePayload, _ = edge.Agent.ProcessPL(finalPl)

	return Result{
		CADID:       edge.Agent.Pw().YouDID(),
		ServiceAddr: eaEnp,
		Invitation:  invit,
	}, nil
}
