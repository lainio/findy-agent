package psm

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/findy-network/findy-common-go/crypto/db"
	"github.com/findy-network/findy-wrapper-go/dto"
	"github.com/go-test/deep"
	"github.com/lainio/err2"
	"github.com/stretchr/testify/assert"
)

const (
	dbPath = "db_test.bolt"
)

func TestMain(m *testing.M) {
	setUp()
	code := m.Run()
	tearDown()
	os.Exit(code)
}

func setUp() {
	defer err2.CatchTrace(func(err error) {
		fmt.Println("error on setup", err)
	})

	// We don't want logs on file with tests
	err2.Check(flag.Set("logtostderr", "true"))

	err2.Check(Open(dbPath))
}

func tearDown() {
	db.Close()

	os.Remove(dbPath)
}

func Test_addPSM(t *testing.T) {
	psm := testPSM(0)
	assert.NotNil(t, psm)

	tests := []struct {
		name string
	}{
		{"add"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddPSM(psm)
			if err != nil {
				t.Errorf("AddPSM() %s error %v", tt.name, err)
			}
			got, err := GetPSM(StateKey{DID: mockStateDID, Nonce: mockStateNonce})
			if diff := deep.Equal(psm, got); err != nil || diff != nil {
				t.Errorf("GetPSM() diff %v, err %v", diff, err)
			}
		})
	}
}

func Test_addPairwiseRep(t *testing.T) {
	pwRep := &PairwiseRep{
		Key:  StateKey{DID: mockStateDID, Nonce: mockStateNonce},
		Name: "name",
	}
	tests := []struct {
		name string
	}{
		{"add"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddPairwiseRep(pwRep)
			if err != nil {
				t.Errorf("AddPairwiseRep() %s error %v", tt.name, err)
			}
			got, err := GetPairwiseRep(pwRep.Key)
			if diff := deep.Equal(pwRep, got); err != nil || diff != nil {
				t.Errorf("GetPairwiseRep() diff %v, err %v", diff, err)
			}
		})
	}
}

type testRep struct {
	RepKey StateKey
}

func (t *testRep) Key() *StateKey {
	return &t.RepKey
}

func (t *testRep) Data() []byte {
	return dto.ToGOB(t)
}

func (t *testRep) Type() byte {
	return BucketPSM // just use any type
}

func NewTestRep(d []byte) Rep {
	p := &testRep{}
	dto.FromGOB(d, p)
	return p
}

func Test_addBaseRep(t *testing.T) {
	msgRep := &testRep{
		RepKey: StateKey{DID: mockStateDID, Nonce: mockStateNonce},
	}
	tests := []struct {
		name string
	}{
		{"add"},
	}
	Creator.Add(BucketPSM, NewTestRep)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddRep(msgRep)
			if err != nil {
				t.Errorf("AddRep() %s error %v", tt.name, err)
			}
			got, err := GetRep(msgRep.Type(), msgRep.RepKey)
			if diff := deep.Equal(msgRep, got); err != nil || diff != nil {
				t.Errorf("GetRep() diff %v, err %v", diff, err)
			}
		})
	}
}
