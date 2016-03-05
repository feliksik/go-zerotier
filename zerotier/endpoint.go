package zerotier

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
)

/**
  FIXME: terminology --  endpoint, member, node -- it's all the same, it seems.
  Ask Adam whether he has a unified terminology we should adhere to.

*/

// helper function:
// run a zerotier-cli command with args, and return  stdout byte buffer.
// if command exists with non-zero status, return stderr output as an error
// it is up to the caller to parse the json output
func ZTClientJson(args ...string) (*bytes.Buffer, error) {
	args = append([]string{"-j"}, args...)
	cmd := exec.Command("zerotier-cli", args...)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stderr = bufio.NewWriter(&errBuf)
	cmd.Stdout = bufio.NewWriter(&outBuf)

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf(string(errBuf.Bytes()))
	}

	return &outBuf, nil
}

// Return an Endpoint struct, with initialized DeviceId
// Assumes the daemon is running.
// fails if the Daemon cannot be reached
func CreateEndpoint() (*Endpoint, error) {
	e := Endpoint{}
	if err := e.UpdateStatus(); err != nil {
		return nil, err
	}

	return &e, nil
}

// endpoint has same methods as zerotier-cli.
// The json representation is a subset of 'zerotier-cli status'
type Endpoint struct {
	DeviceAddress string          `json:"address"`
	data          json.RawMessage ``
	Online        bool            `json:"online"`
	TCPFallback   bool            `json:"tcpFallbackActive"`
}

// start the daemon, if it is not already running
// Note: you probably shouldn't use this function, but instead start it with systemd or so.
func StartDaemon() error {
	if PingDaemon() != nil {
		//start zerotier service

		// @FIXME is this a resilient way to start it? Maybe a service call?
		cmd := exec.Command("/var/lib/zerotier-one/zerotier-one")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			return err
		}
		//@todo find a more elegant solution to wait for zerotier to be up
		<-time.After(time.Second)
		if PingDaemon() != nil {
			return fmt.Errorf("Cannot connect to local zt daemon")
		}
	}
	return nil
}

// check if we can reach the daemon.
// If not, it may not be running, we may not have permission to read the token, or
// there may be some other problem.
func PingDaemon() error {
	_, err := ZTClientJson("status")
	return err
}

func (e *Endpoint) UpdateStatus() error {
	result, err := ZTClientJson("status")
	if err != nil {
		return err
	}
	err = json.Unmarshal(result.Bytes(), &e)
	if err != nil {
		return err
	}
	return nil
}

// join the network; this sets up a connection, but doesn't authenticate
// that means we are not sure if we will get an IP
func (e *Endpoint) Join(networkId string) error {
	//start and join zerotier network
	_, err := ZTClientJson("join", networkId)
	return err
}

// leave a network
func (e *Endpoint) Leave(networkId string) error {
	//start and join zerotier network
	_, err := ZTClientJson("leave", networkId)
	return err
}

// get a list of Network structs.
// They are freshly updated by querying the endpoint daemon.
func (e *Endpoint) ListNetworks() map[string]Network {
	dataBuf, err := ZTClientJson("listnetworks")
	if err != nil {
		return nil // should we propagate error with 2 args?
	}

	var networkList []Network
	data := dataBuf.Bytes()
	if err := json.Unmarshal(data, &networkList); err != nil {
		log.Printf("Warning: cannot parse getnetworks output: \n")
		log.Printf("Network data: %s \n\n Error is: ", string(data))
		log.Printf(err.Error())
		return nil
	}

	networkMap := make(map[string]Network)
	for _, nw := range networkList {
		networkMap[nw.Id] = nw
	}
	// net.ParseCIDR(addrs[0].String())
	return networkMap
}

// get network with given id. If it doesn't exist, return nil.
func (e *Endpoint) GetNetwork(id string) *Network {
	nets := e.ListNetworks()
	n, ok := nets[id]

	if ok {
		return &n // n is non-nil, return useful struct
	} else {
		return nil // n is empty struct, return nil
	}

}

// wait until a network address being assigned to the vpn interface.
// This call blocks until an ip address is detected, or the exit channel reads a signal.
//
// precondition: we joined a network, and it is authorized by the ZT controller
func (e *Endpoint) WaitForIP(exit chan os.Signal, networkId string) (net.IP, error) {

	for {
		nw := e.GetNetwork(networkId)
		if nw == nil {
			return nil, fmt.Errorf("no such network: " + networkId)
		}

		addrs := nw.Addresses

		if len(addrs) > 0 {
			ip, _, err := net.ParseCIDR(addrs[0])
			if err != nil {
				return nil, err
			}

			if ip.To4() != nil {
				return ip, nil
			} else {
				// we cannot handle ipv6 ?
				return nil, fmt.Errorf("Cannot handle ip address as ipv4: " + addrs[0])
			}
		}

		select {
		case <-exit:
			return nil, fmt.Errorf("User cancelled")
		case <-time.After(time.Second * 1):
		}
	}
}

type Network struct {
	Id               string `json:"nwid"`
	Mac              string `json:"mac"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Type             string `json:"type"`
	MTU              int    `json:"mtu"`
	DCHP             bool   `json:"dhcp"`
	Bridge           bool   `json:"bridge"`
	BroadcastEnabled bool   `json:"broadcastEnabled"`
	//PortError int `json:"portError"`
	//netconfRevision int `json:"netconfRevision"`
	// multicastSubscriptions ignored
	Addresses      []string `json:"assignedAddresses"`
	PortDeviceName string   `json:"portDeviceName"`
}
