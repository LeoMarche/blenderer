package node

import (
	"sync"

	"github.com/LeoMarche/blenderer/src/utils"
)

//Node is the base descriptor of a Node
type Node struct {
	Name   string `json:"name"`
	IP     string `json:"ip"`
	APIKey string `json:"api_key"`
	state  string
	sync.Mutex
}

//State returns the state of the Node N
func (n *Node) State() string {

	return n.state
}

//Commission allows to assign a node to a render task it allows concurrency. It returns true if comission was succesful and false else
func (n *Node) Commission() bool {

	ret := false

	//Locking node during assignement (no concurrent assignment)
	n.Lock()
	defer n.Unlock()

	if n.state == "available" {
		n.state = "rendering"
		ret = true
	}

	return ret
}

//SetState can be used to st the state of a Node to an accepted value
func (n *Node) SetState(state string) bool {
	acceptedStates := []string{"available", "rendering", "down", "error"}
	if utils.IsIn(state, acceptedStates) >= 0 {
		n.Lock()
		n.state = state
		n.Unlock()
		return true
	}

	return false
}

//Free sets the state of the node to available
func (n *Node) Free() {

	n.Lock()
	defer n.Unlock()
	if n.state == "rendering" {
		n.state = "available"
	}

}

//Up allows to reactivate a down node
func (n *Node) Up() {
	n.Lock()
	defer n.Unlock()

	if n.state == "down" {
		n.state = "available"
	}
}
