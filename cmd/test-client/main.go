package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
)

func main() {
	// 1. Start the server
	cmd := exec.Command("./mcp-lmstudio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	defer cmd.Process.Kill()

	reader := bufio.NewReader(stdout)

	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Printf("SENT: %s\n", string(b))
		stdin.Write(append(b, '\n'))
	}

	receive := func() map[string]any {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Fatal(err)
		}
		fmt.Printf("RECV: %s\n", string(line))
		var res map[string]any
		json.Unmarshal(line, &res)
		return res
	}

	// 2. Step 1: Initialize
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-client", "version": "1.0.0"},
		},
		"id": 1,
	})

	// Wait for initialize response
	for {
		resp := receive()
		if resp["id"] == float64(1) {
			break
		}
	}

	// 3. Step 2: Initialized notification
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})

	// 4. Step 3: Call tool (health_check)
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "health_check",
			"arguments": map[string]any{},
		},
		"id": 2,
	})

	// Wait for tool result
	for {
		resp := receive()
		if resp["id"] == float64(2) {
			fmt.Println("\nSUCCESS! health_check output received.")
			result, _ := json.MarshalIndent(resp["result"], "", "  ")
			fmt.Println(string(result))
			break
		}
	}

	// 5. Step 4: Call tool (list_models)
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_models",
			"arguments": map[string]any{},
		},
		"id": 3,
	})

	// Wait for tool result
	for {
		resp := receive()
		if resp["id"] == float64(3) {
			fmt.Println("\nSUCCESS! list_models output received.")
			result, _ := json.MarshalIndent(resp["result"], "", "  ")
			fmt.Println(string(result))
			break
		}
	}

	// 6. Step 5: Call tool (get_current_model)
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_current_model",
			"arguments": map[string]any{},
		},
		"id": 4,
	})

	// Wait for tool result
	for {
		resp := receive()
		if resp["id"] == float64(4) {
			fmt.Println("\nSUCCESS! get_current_model output received.")
			result, _ := json.MarshalIndent(resp["result"], "", "  ")
			fmt.Println(string(result))
			break
		}
	}

	// 7. Step 6: Call tool (chat_completion)
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "chat_completion",
			"arguments": map[string]any{
				"prompt":      "What is 2+2? Answer in one sentence.",
				"temperature": 0.7,
				"max_tokens":  50,
			},
		},
		"id": 5,
	})

	// Wait for tool result
	for {
		resp := receive()
		if resp["id"] == float64(5) {
			fmt.Println("\nSUCCESS! chat_completion output received.")
			result, _ := json.MarshalIndent(resp["result"], "", "  ")
			fmt.Println(string(result))
			break
		}
	}

	fmt.Println("\n=== ALL TESTS COMPLETED SUCCESSFULLY ===")
}
