package iptables

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

const chainName = "DOCKEREST"

var natChain *ChainInfo
var filterChain *ChainInfo
var bridgeName string

func TestNewChain(t *testing.T) {
	var err error

	bridgeName = "lo"
	iptable := GetIptable(IPv4)

	natChain, err = iptable.NewChain(chainName, Nat, false)
	if err != nil {
		t.Fatal(err)
	}
	err = iptable.ProgramChain(natChain, bridgeName, false, true)
	if err != nil {
		t.Fatal(err)
	}

	filterChain, err = iptable.NewChain(chainName, Filter, false)
	if err != nil {
		t.Fatal(err)
	}
	err = iptable.ProgramChain(filterChain, bridgeName, false, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestForward(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	port := 1234
	dstAddr := "172.17.0.1"
	dstPort := 4321
	proto := "tcp"

	bridgeName := "lo"
	iptable := GetIptable(IPv4)

	err := natChain.Forward(Insert, ip, port, proto, dstAddr, dstPort, bridgeName)
	if err != nil {
		t.Fatal(err)
	}

	dnatRule := []string{
		"-d", ip.String(),
		"-p", proto,
		"--dport", strconv.Itoa(port),
		"-j", "DNAT",
		"--to-destination", dstAddr + ":" + strconv.Itoa(dstPort),
		"!", "-i", bridgeName,
	}

	if !iptable.Exists(natChain.Table, natChain.Name, dnatRule...) {
		t.Fatal("DNAT rule does not exist")
	}

	filterRule := []string{
		"!", "-i", bridgeName,
		"-o", bridgeName,
		"-d", dstAddr,
		"-p", proto,
		"--dport", strconv.Itoa(dstPort),
		"-j", "ACCEPT",
	}

	if !iptable.Exists(filterChain.Table, filterChain.Name, filterRule...) {
		t.Fatal("filter rule does not exist")
	}

	masqRule := []string{
		"-d", dstAddr,
		"-s", dstAddr,
		"-p", proto,
		"--dport", strconv.Itoa(dstPort),
		"-j", "MASQUERADE",
	}

	if !iptable.Exists(natChain.Table, "POSTROUTING", masqRule...) {
		t.Fatal("MASQUERADE rule does not exist")
	}
}

func TestLink(t *testing.T) {
	var err error

	bridgeName := "lo"
	iptable := GetIptable(IPv4)
	ip1 := net.ParseIP("192.168.1.1")
	ip2 := net.ParseIP("192.168.1.2")
	port := 1234
	proto := "tcp"

	err = filterChain.Link(Append, ip1, ip2, port, proto, bridgeName)
	if err != nil {
		t.Fatal(err)
	}

	rule1 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip1.String(),
		"-d", ip2.String(),
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT"}

	if !iptable.Exists(filterChain.Table, filterChain.Name, rule1...) {
		t.Fatal("rule1 does not exist")
	}

	rule2 := []string{
		"-i", bridgeName,
		"-o", bridgeName,
		"-p", proto,
		"-s", ip2.String(),
		"-d", ip1.String(),
		"--sport", strconv.Itoa(port),
		"-j", "ACCEPT"}

	if !iptable.Exists(filterChain.Table, filterChain.Name, rule2...) {
		t.Fatal("rule2 does not exist")
	}
}

func TestPrerouting(t *testing.T) {
	args := []string{
		"-i", "lo",
		"-d", "192.168.1.1"}
	iptable := GetIptable(IPv4)

	err := natChain.Prerouting(Insert, args...)
	if err != nil {
		t.Fatal(err)
	}

	if !iptable.Exists(natChain.Table, "PREROUTING", args...) {
		t.Fatal("rule does not exist")
	}

	delRule := append([]string{"-D", "PREROUTING", "-t", string(Nat)}, args...)
	if _, err = iptable.Raw(delRule...); err != nil {
		t.Fatal(err)
	}
}

func TestOutput(t *testing.T) {
	args := []string{
		"-o", "lo",
		"-d", "192.168.1.1"}
	iptable := GetIptable(IPv4)

	err := natChain.Output(Insert, args...)
	if err != nil {
		t.Fatal(err)
	}

	if !iptable.Exists(natChain.Table, "OUTPUT", args...) {
		t.Fatal("rule does not exist")
	}

	delRule := append([]string{"-D", "OUTPUT", "-t",
		string(natChain.Table)}, args...)
	if _, err = iptable.Raw(delRule...); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrencyWithWait(t *testing.T) {
	RunConcurrencyTest(t, true)
}

func TestConcurrencyNoWait(t *testing.T) {
	RunConcurrencyTest(t, false)
}

// Runs 10 concurrent rule additions. This will fail if iptables
// is actually invoked simultaneously without --wait.
// Note that if iptables does not support the xtable lock on this
// system, then allowXlock has no effect -- it will always be off.
func RunConcurrencyTest(t *testing.T, allowXlock bool) {
	var wg sync.WaitGroup

	if !allowXlock && supportsXlock {
		supportsXlock = false
		defer func() { supportsXlock = true }()
	}

	ip := net.ParseIP("192.168.1.1")
	port := 1234
	dstAddr := "172.17.0.1"
	dstPort := 4321
	proto := "tcp"

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := natChain.Forward(Append, ip, port, proto, dstAddr, dstPort, "lo")
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestCleanup(t *testing.T) {
	var err error
	var rules []byte

	// Cleanup filter/FORWARD first otherwise output of iptables-save is dirty
	link := []string{"-t", string(filterChain.Table),
		string(Delete), "FORWARD",
		"-o", bridgeName,
		"-j", filterChain.Name}
	iptable := GetIptable(IPv4)

	if _, err = iptable.Raw(link...); err != nil {
		t.Fatal(err)
	}
	filterChain.Remove()

	err = iptable.RemoveExistingChain(chainName, Nat)
	if err != nil {
		t.Fatal(err)
	}

	rules, err = exec.Command("iptables-save").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rules), chainName) {
		t.Fatalf("Removing chain failed. %s found in iptables-save", chainName)
	}
}

func TestExistsRaw(t *testing.T) {
	testChain1 := "ABCD"
	testChain2 := "EFGH"

	iptable := GetIptable(IPv4)

	_, err := iptable.NewChain(testChain1, Filter, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		iptable.RemoveExistingChain(testChain1, Filter)
	}()

	_, err = iptable.NewChain(testChain2, Filter, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		iptable.RemoveExistingChain(testChain2, Filter)
	}()

	// Test detection over full and truncated rule string
	input := []struct{ rule []string }{
		{[]string{"-s", "172.8.9.9/32", "-j", "ACCEPT"}},
		{[]string{"-d", "172.8.9.0/24", "-j", "DROP"}},
		{[]string{"-s", "172.0.3.0/24", "-d", "172.17.0.0/24", "-p", "tcp", "-m", "tcp", "--dport", "80", "-j", testChain2}},
		{[]string{"-j", "RETURN"}},
	}

	for i, r := range input {
		ruleAdd := append([]string{"-t", string(Filter), "-A", testChain1}, r.rule...)
		err = iptable.RawCombinedOutput(ruleAdd...)
		if err != nil {
			t.Fatalf("i=%d, err: %v", i, err)
		}
		if !iptable.exists(true, Filter, testChain1, r.rule...) {
			t.Fatalf("Failed to detect rule. i=%d", i)
		}
		// Truncate the rule
		trg := r.rule[len(r.rule)-1]
		trg = trg[:len(trg)-2]
		r.rule[len(r.rule)-1] = trg
		if iptable.exists(true, Filter, testChain1, r.rule...) {
			t.Fatalf("Invalid detection. i=%d", i)
		}
	}
}
