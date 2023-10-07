package gossip

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"

	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

/////////////////////////////////////////////////////////////////
// Error handlers
/////////////////////////////////////////////////////////////////
var (
	MALFORMED_MESSAGE_TYPE = func(data any) error { return fmt.Errorf("Malformed input for message feild %v", data)}
	MALFORMED_MESSAGE_BODY = func(feild string) error { return fmt.Errorf("Malformed input for message body no feild: %v", feild)}
)

type Gossip struct{
	messages []float64
	topology map[string]any
	Node *maelstrom.Node
	lock sync.Mutex
	log *log.Logger
}

func NewGossipHandler(node *maelstrom.Node, file *os.File) *Gossip{
	l := log.Default()
	l.SetOutput(file)
	return &Gossip{
		messages: make([]float64, 0),
		topology: make(map[string]any),
		Node: node,
		log: l,
	}
}

func (g *Gossip) Broadcast(msg maelstrom.Message) error {
	// Extract the body
	var body map[string]any
	response := make(map[string]any)
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message, ok := body["message"]

	if !ok {
		return MALFORMED_MESSAGE_BODY("message")
	}

	g.log.Printf("The message is %v",message)
	var intValue float64

	switch t := message.(type){
	case int:
		intValue = float64(t)
	case string:{
		///////////////////////////////////////////
		// doing this conversion because just int cast was failing
		//////////////////////////////////////////////////////////
		interimInt, err := strconv.Atoi(t)
		if err != nil {
			g.log.Printf("mesage value is bad %v",err)
			return MALFORMED_MESSAGE_TYPE(t)
		}
		intValue = float64(interimInt)
	}
	case float64:
		intValue = t
	case float32:
		intValue = float64(t)
	default:{
			g.log.Printf("Unexpected type of message %v",reflect.TypeOf(message))
			return MALFORMED_MESSAGE_TYPE(t)
	}

	}

	response["type"] = "broadcast_ok"
	response["msg_id"] = body["msg_id"]
	////////////////////////////////////////////
	// Mutex is being used here so that shared
	// resources are accessed properly
	///////////////////////////////////////////
	g.lock.Lock()
	g.messages = append(g.messages, intValue)
	g.lock.Unlock()
	return g.Node.Reply(msg, response)
}

func (g *Gossip) Read(msg maelstrom.Message) error {
	// Extract the body
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "read_ok"
	////////////////////////////////////////////
	// Mutex is being used here so that shared
	// resources are accessed properly
	///////////////////////////////////////////
	g.lock.Lock()
	body["messages"] = g.messages
	g.lock.Unlock()

	return g.Node.Reply(msg, body)
}


func (g *Gossip) Topology(msg maelstrom.Message) error {
	// Extract the body
	var body map[string]any
	response := make(map[string]any)
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message, ok := body["topology"]

	if !ok {
		g.log.Printf("Topology malformed body")
		return MALFORMED_MESSAGE_BODY("topology")
	}

	topology, ok := message.(map[string]any)

	if !ok {
		g.log.Printf("Topology malformed message")
		return MALFORMED_MESSAGE_TYPE(topology)
	}
	response["type"] = "topology_ok"
	response["msg_id"] = body["msg_id"]
	////////////////////////////////////////////
	// Mutex is being used here so that shared
	// resources are accessed properly
	///////////////////////////////////////////
	g.lock.Lock()
	g.topology = topology
	g.lock.Unlock()
	// delete(body,"topology")

	return g.Node.Reply(msg, response)
}