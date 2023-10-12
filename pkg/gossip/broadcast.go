package gossip

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

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

	///////////////////////////////////////////
	// send message to the other nodes
	//////////////////////////////////////////
	nodes := make(map[string]bool)
	respChan := make(chan ReturnMessage)
	retryLimit := 10
	for k := range g.topology{
		
		if err := g.Node.RPC(k,body,g.RetryBroadCastHandler(respChan, 0, retryLimit)); err != nil {
			return err
		}
		nodes[k] = false
	}
	ctx , _ := context.WithTimeout(context.Background(), 5*time.Second)
	go g.RetryResponseHandler(respChan, ctx,nodes)

	return g.Node.Reply(msg, response)
}

type ReturnMessage struct{
	Message maelstrom.Message
	RetryCount int
	RetryLimit int
}

func (g *Gossip) RetryBroadCastHandler(respChan chan <- ReturnMessage, retryCount int, retryLimit int) maelstrom.HandlerFunc {
	return func(msg maelstrom.Message) error {
		response := ReturnMessage{
			Message: msg,
			RetryCount: retryCount,
			RetryLimit: retryLimit,
		}
		respChan <- response
		return nil
	}
}

func (g *Gossip) RetryResponseHandler(respChan chan ReturnMessage, ctx context.Context, nodes map[string]bool) {
	for {
		select{
		case <- ctx.Done():{
			g.log.Println("Response handler timed out!")
			return
		}
		case r := <- respChan: {
			var body map[string]any
			var rpcErr maelstrom.RPCError
			if rpcErr := r.Message.RPCError(); rpcErr == nil {
				g.log.Printf("The %s source recieved message successfully.", r.Message.Src)
				nodes[r.Message.Src] = true
				break
			}
			// If there is an error with the bad message body retry will not take place.
			if rpcErr.Code == 13 {
				g.log.Printf("The returned message, caused an error from %s => %#v",r.Message.Src, rpcErr.Text)
				nodes[r.Message.Src] = true
				break
			}
			// Im currently discarding bad responses
			if r.RetryCount < r.RetryLimit {
				g.log.Printf("Retrying to node %s from %s",r.Message.Src,g.Node.ID())
				_ = json.Unmarshal(r.Message.Body, &body)
				go func(body map[string]any, src string, respChan chan <- ReturnMessage, retryCount int, retryLimit int){
					time.Sleep(100*time.Millisecond)
					g.Node.RPC(src,body,g.RetryBroadCastHandler(respChan, retryCount, retryLimit))
				}(body, r.Message.Src, respChan, (r.RetryCount + 1), r.RetryLimit)
			}
		}
		}
		g.log.Printf("The node %s status of broadcast %#v",g.Node.ID(),nodes)
		overAllStatus := true
		for _, status :=  range nodes {
			overAllStatus = overAllStatus && status
		}

		if overAllStatus {
			g.log.Printf("All the messages were sent %#v",nodes)
			return
		}

	}
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