package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComission(t *testing.T) {

	assert := assert.New(t)

	n1 := Node{
		Name:   "test_name",
		IP:     "test_ip",
		APIKey: "test_a_k",
		state:  "available",
	}

	n2 := Node{
		Name:   "test_name",
		IP:     "test_ip",
		APIKey: "test_a_k",
		state:  "rendering",
	}

	var t0 bool
	go func() {
		t0 = n1.Commission()
	}()

	go n1.Commission()

	t1 := n1.Commission()
	t2 := n2.Commission()

	assert.NotEqual(t0, t1, "Node comissioned by two concurrent processes")
	assert.Equal(n1.state, "rendering", "Not comissioning an available node")
	assert.Equal(t2, false, "Return comissioning of not available node")
	assert.Equal(n2.state, "rendering", "Comissioning not available node")
}

func TestState(t *testing.T) {

	assert := assert.New(t)

	n1 := Node{
		Name:   "test_name",
		IP:     "test_ip",
		APIKey: "test_a_k",
		state:  "available",
	}

	t0 := n1.State()
	n1.state = "rendering"
	t1 := n1.State()
	n1.SetState("down")
	t2 := n1.State()
	n1.SetState("coucou")
	t3 := n1.State()

	assert.Equal(t0, "available", "Bad state returned")
	assert.Equal(t1, "rendering", "Bad state returned after change")
	assert.Equal(t2, "down", "SetState doesn't work")
	assert.Equal(t3, "down", "SetState allows non-usable states")

}

func TestFree(t *testing.T) {

	assert := assert.New(t)

	n1 := Node{
		Name:   "test_name",
		IP:     "test_ip",
		APIKey: "test_a_k",
		state:  "rendering",
	}

	n1.Free()
	t0 := n1.state
	n1.state = "down"
	n1.Free()
	t1 := n1.state

	assert.Equal(t0, "available", "Node freed a rendering node")
	assert.Equal(t1, "down", "Freed a down node")
}

func TestUp(t *testing.T) {

	assert := assert.New(t)

	n1 := Node{
		Name:   "test_name",
		IP:     "test_ip",
		APIKey: "test_a_k",
		state:  "rendering",
	}

	n1.Up()
	t0 := n1.state
	n1.state = "down"
	n1.Up()
	t1 := n1.state

	assert.Equal(t0, "rendering", "Uping a rendering node")
	assert.Equal(t1, "available", "Not uping a down node")
}
