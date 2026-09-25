// Design: docs/architecture/testing/interop.md -- Credentialed native L2TP acceptance

package testdeployment

import "testing"

func TestCHAPRefusalRequiresTheAnsweredChallenge(t *testing.T) {
	challenge := "rcvd [CHAP Challenge id=0x9 <abcd>, name = ze]\n"
	response := "sent [CHAP Response id=0x9 <dcba>, name = alice]\n"
	failure := "rcvd [CHAP Failure id=0x9 \"bad CHAP response\"]\n"
	for _, test := range []struct {
		name     string
		log      string
		rejected bool
	}{
		{"answered challenge rejected", challenge + response + failure, true},
		{"timeout", challenge + response, false},
		{"unanswered challenge", challenge + failure, false},
		{"unsolicited failure", failure, false},
		{"response before challenge", response + challenge + failure, false},
		{"failure for old challenge", challenge + response + "rcvd [CHAP Failure id=0x8]\n", false},
		{"response for old challenge", challenge + "sent [CHAP Response id=0x8 <dcba>, name = alice]\n" + failure, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := l2tpPPPCHAPRejected(test.log)
			if err != nil || got != test.rejected {
				t.Fatalf("rejected = %v, error = %v; want %v", got, err, test.rejected)
			}
		})
	}
}

func TestCHAPRefusalCannotHideNetworkAdmission(t *testing.T) {
	rejected := "rcvd [CHAP Challenge id=0x9 <abcd>, name = ze]\n" +
		"sent [CHAP Response id=0x9 <dcba>, name = alice]\n" +
		"rcvd [CHAP Failure id=0x9 \"bad CHAP response\"]\n"
	for _, forbidden := range []string{
		"sent [IPCP ConfReq id=0xa <addr 10.100.0.2>]\n",
		"rcvd [IPCP ConfReq id=0xb <addr 10.100.0.1>]\n",
		"rcvd [CHAP Success id=0x9]\n",
		"local  IP address 10.100.0.2\n",
	} {
		for _, log := range []string{forbidden + rejected, rejected + forbidden} {
			if accepted, err := l2tpPPPCHAPRejected(log); err == nil || accepted {
				t.Fatalf("admitted peer passed the rejection proof: rejected = %v, error = %v, log:\n%s", accepted, err, log)
			}
		}
	}
}
