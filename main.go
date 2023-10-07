package main

import (
	"encoding/json"
	"log"
	"malestrom-echo/pkg/gossip"
	"math/rand"
	"os"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()

	n.Handle("echo", func(msg maelstrom.Message) error {
		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "echo_ok"

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	n.Handle("generate", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		body["type"] = "generate_ok"
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)

		body["id"] = r1.Int()

		return n.Reply(msg, body)

	})

	/////////////////////////////////////////////////////////////////////////////////////////
	// logger
	////////////////////////////////////////////////////////////////////////////////////////
	f, err := os.OpenFile("/tmp/testlogfile", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)

	if err != nil {
		log.Fatalf("File testlogfile not created!")
	}
	defer f.Close()
	//////////////////////////////////////////////////////////////////////////////////////////
	// Handling Gossip messages
	/////////////////////////////////////////////////////////////////////////////////////////
	g := gossip.NewGossipHandler(n, f)
	n.Handle("broadcast", g.Broadcast)
	n.Handle("read",g.Read)
	n.Handle("topology", g.Topology)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}

}
