package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("MCP LM Studio Test Client")
	fmt.Println("==================================================")
	fmt.Println("NOTE: LM Studio must be running on http://127.0.0.1:1234")
	fmt.Println("      with a model loaded for tests to pass.")
	fmt.Println("==================================================\n")

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
	testsFailed := false

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

	checkError := func(resp map[string]any, testName string) bool {
		if errObj, hasError := resp["error"]; hasError {
			fmt.Printf("\n❌ FAILED! %s returned an error:\n", testName)
			errorJSON, _ := json.MarshalIndent(errObj, "", "  ")
			fmt.Println(string(errorJSON))
			testsFailed = true
			return true
		}
		return false
	}

	checkResult := func(resp map[string]any, testName string) bool {
		result, hasResult := resp["result"].(map[string]any)
		if !hasResult {
			fmt.Printf("\n❌ FAILED! %s has no result field\n", testName)
			testsFailed = true
			return false
		}

		content, hasContent := result["content"].([]any)
		if !hasContent || len(content) == 0 {
			fmt.Printf("\n❌ FAILED! %s has no content\n", testName)
			testsFailed = true
			return false
		}

		textContent, ok := content[0].(map[string]any)
		if !ok {
			fmt.Printf("\n❌ FAILED! %s content is not properly formatted\n", testName)
			testsFailed = true
			return false
		}

		text, ok := textContent["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			fmt.Printf("\n❌ FAILED! %s has empty text content\n", testName)
			testsFailed = true
			return false
		}

		// Check for error messages in the response text
		lowerText := strings.ToLower(text)
		if strings.Contains(lowerText, "error") || strings.Contains(lowerText, "failed") || strings.Contains(lowerText, "connection refused") {
			fmt.Printf("\n❌ FAILED! %s returned an error:\n%s\n", testName, text)
			testsFailed = true
			return false
		}

		return true
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
			if checkError(resp, "health_check") {
				break
			}
			if checkResult(resp, "health_check") {
				fmt.Println("\n✅ SUCCESS! health_check passed.")
				result, _ := json.MarshalIndent(resp["result"], "", "  ")
				fmt.Println(string(result))
			}
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
			if checkError(resp, "list_models") {
				break
			}
			if checkResult(resp, "list_models") {
				fmt.Println("\n✅ SUCCESS! list_models passed.")
				result, _ := json.MarshalIndent(resp["result"], "", "  ")
				fmt.Println(string(result))
			}
			break
		}
	}

	// 6. Step 5: Call tool (chat)
	send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "chat",
			"arguments": map[string]any{
				"prompt":         "What is 2+2? Answer in one sentence.",
				"temperature":    0.7,
				"context_length": 8000,
			},
		},
		"id": 4,
	})

	// Wait for tool result
	for {
		resp := receive()
		if resp["id"] == float64(4) {
			if checkError(resp, "chat") {
				break
			}
			if checkResult(resp, "chat") {
				fmt.Println("\n✅ SUCCESS! chat passed.")
				result, _ := json.MarshalIndent(resp["result"], "", "  ")
				fmt.Println(string(result))
			}
			break
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	if testsFailed {
		fmt.Println("❌ SOME TESTS FAILED")
		fmt.Println(strings.Repeat("=", 50))
		os.Exit(1)
	} else {
		fmt.Println("✅ ALL TESTS PASSED SUCCESSFULLY")
		fmt.Println(strings.Repeat("=", 50))
	}
}
