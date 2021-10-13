package endp

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/findy-network/findy-agent/agent/service"
)

/*
Addr is one of the central types in the Findy Agency. It handles the connections
both for server and client. It caries mostly the URL specific stuff and DID
parsing logic. There is also WebSocket connection. In the future there might be
some other elements as well. This is right place. The name of the type should be
changed to something else like CnxAddr, end need for exportation should be
considered.
*/
type Addr struct {
	ID        uint64 // ID is used to save Payloads arriving into the Address
	Service   string // Service name like findy for http
	PlRcvr    string // PL receiver it is CA-DID (our CA) todo: CA or WA?
	MsgRcvr   string // Receivers CA-DID which is used for Edge/Cloud communication
	RcvrDID   string // This is the pairwise DID
	EdgeToken string // Final communication endpoint, now used for invitation ID
	BasePath  string // The base address of the URL
	VerKey    string // Associated VerKey, used for sending Payloads to this address
}

const DIDLengthMax = 22
const DIDLengthMin = 21

var r *regexp.Regexp

func init() {
	// Regexp rule from:
	// https://sovrin-foundation.github.io/sovrin/spec/did-method-spec-template.html#namespace-specific-identifier-nsi
	expr := fmt.Sprintf("^[1-9A-HJ-NP-Za-km-z]{%d,%d}", DIDLengthMin, DIDLengthMax)

	r, _ = regexp.Compile(expr)
}

// NewServerAddr creates and fills new object from string usually got from
// service calls like HTTP POST request. For that reason it cannot fill base
// address field.
func NewServerAddr(s string) (ea *Addr) {
	ea = new(Addr)
	parts := strings.Split(s, "/")
	for i, part := range parts {
		switch i {
		case 1:
			ea.Service = part
		case 2:
			ea.PlRcvr = part
		case 3:
			ea.MsgRcvr = part
		case 4:
			ea.RcvrDID = part
		case 5:
			ea.EdgeToken = part
		}
	}
	return
}

// NewClientAddr creates and fills new object from string which holds full URL
// of the address, including base address as well. This can and should be used
// for cases where whole endpoint address is given.
func NewClientAddr(s string) (ea *Addr) {
	ea = new(Addr)
	u, _ := url.Parse(s)
	ea.BasePath = u.Scheme + "://" + u.Host
	parts := strings.Split(u.Path, "/")
	for i, part := range parts {
		switch i {
		case 1:
			ea.Service = part
		case 2:
			ea.PlRcvr = part
		case 3:
			ea.MsgRcvr = part
		case 4:
			ea.RcvrDID = part
		case 5:
			ea.EdgeToken = part
		}
	}
	return
}

func NewClientAddrWithKey(fullURL, verkey string) *Addr {
	addr := NewClientAddr(fullURL)
	addr.VerKey = verkey
	return addr
}

func NewAddrFromPublic(ae service.Addr) *Addr {
	return NewClientAddrWithKey(ae.Endp, ae.Key)
}

func (e *Addr) Valid() bool {
	if !IsInEndpoints(e.PlRcvr) {
		return IsDID(e.PlRcvr) && IsDID(e.MsgRcvr) && IsDID(e.RcvrDID)
	}
	return true
}

func IsDID(DID string) bool {
	l := len(DID)
	lenOK := DIDLengthMin <= l && l <= DIDLengthMax
	return lenOK && r.MatchString(DID)
}

// ReceiverDID returns actual agent PL receiver.
func (e *Addr) ReceiverDID() string {
	if e.MsgRcvr == "" {
		return e.PlRcvr
	}
	return e.MsgRcvr
}

// MsgLevelDID returns the actual agent who receives and handles the PL. Note!
// it's safer to use RcvrDID field instead!
func (e *Addr) MsgLevelDID() string {
	if e.RcvrDID != "" {
		return e.RcvrDID
	}
	return e.ReceiverDID()
}

func (e *Addr) Address() string {
	basePath := fmt.Sprintf("%s/%s/%s", e.BasePath, e.Service, e.PlRcvr)
	if e.MsgRcvr != "" {
		basePath += "/" + e.MsgRcvr
	}
	if e.RcvrDID != "" {
		basePath += "/" + e.RcvrDID
	}
	if e.EdgeToken != "" {
		basePath += "/" + e.EdgeToken
	}
	return strings.TrimSuffix(basePath, "/")
}

func (e *Addr) IsEncrypted() bool {
	return !IsInEndpoints(e.PlRcvr)
}

func IsInEndpoints(endpointName string) bool {
	for _, name := range endpoints() {
		if name == endpointName {
			return true
		}
	}
	return false
}

func endpoints() []string {
	return []string{"ping", "handshake"}
}

func (e *Addr) PayloadTransportDID() string {
	if e.IsEncrypted() {
		return e.PlRcvr
	}
	panic("programming error, we should not be here!")
}

func (e *Addr) String() string {
	return e.Address()
}

func (e *Addr) TestAddress() string {
	basePath := fmt.Sprintf("/%s/%s", e.Service, e.PlRcvr)

	if e.MsgRcvr != "" {
		basePath += "/" + e.MsgRcvr
	}
	if e.RcvrDID != "" {
		basePath += "/" + e.RcvrDID
	}
	return strings.TrimSuffix(basePath, "/")
}

// AE returns Addr which includes URL + VerKey.
func (e *Addr) AE() service.Addr {
	return service.Addr{Endp: e.Address(), Key: e.VerKey}
}

func (e *Addr) Key() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, e.ID)
	return b
}
