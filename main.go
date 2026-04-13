package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.design/x/clipboard"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Tools map[string]interface{} `json:"tools"`
}
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type InputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type ToolListResult struct {
	Tools []Tool `json:"tools"`
}

type CallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type CallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError"`
}

var logger *log.Logger

func init() {
	logger = log.New(os.Stderr, "[Clipboard-MCP]", log.Ltime)
}

func main() {
	logger.Println("Starting Clipboard-MCP Server...")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			logger.Printf("Failed to unmarshal request: %v", err)
			continue
		}

		logger.Printf("Recieved method: %v", req.Method)
		dispatchRequest(req)
	}
	if err := scanner.Err(); err != nil {
		logger.Fatalf("Scanner error: %v", err)
	}
}

func handleToolList(id interface{}) {
	result := ToolListResult{
		Tools: []Tool{
			{
				Name:        "read_clipboard_text",
				Description: "Reads the current text contents of the user's Windows clipboard. Use this when the user asks you to look at copied text, code, or error messages.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: make(map[string]PropertySchema),
					Required:   []string{},
				},
			},
			{
				Name:        "write_to_clipboard_text",
				Description: "Write the text to user's Windows clipboard, Use this when the user ask you to copy respose, text, code, or error messages.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"text_to_copy": {
							Type:        "string",
							Description: "The exact text or code snippet to place into the clipboard",
						},
					},
					Required: []string{"text_to_copy"},
				},
			},
			{
				Name:        "read_clipboard_image",
				Description: "Reads the image currently copied in user's Windows clipboard. returned a base64",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: make(map[string]PropertySchema),
					Required:   []string{},
				},
			},
			{
				Name:        "write_to_clipboard_image",
				Description: "Writes a base64 enacoded png image to Windows clipboard.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]PropertySchema{
						"base64_png": {
							Type:        "string",
							Description: "base64 encoded png image",
						},
					},
					Required: []string{"base64_png"},
				},
			},
		},
	}
	writeResponse(id, result)
}

func dispatchRequest(req Request) {
	switch req.Method {
	case "initialize":
		logger.Printf("initialized")
		handleInitialize(req.ID)
	case "tools/list":
		logger.Printf("List of tools")
		handleToolList(req.ID)
	case "tools/call":
		logger.Printf("execute call")
		handleToolCall(req.ID, req.Params)
	case "notifications/initialized":
		logger.Printf("Host confirmed handshake")
	default:
		logger.Printf("unknown method: %s", req.Method)
	}
}

func handleToolCall(id interface{}, rawMessage json.RawMessage) {
	var params CallParams
	if err := json.Unmarshal(rawMessage, &params); err != nil {
		logger.Printf("Failed to unmarshal tool call params: %v", err)
		return
	}
	switch params.Name {
	case "read_clipboard_text":
		text, err := readClipboardText()
		writeResponse(id, CallResult{
			Content: []ToolContent{
				{
					Type: "text",
					Text: text,
				},
			},
			IsError: err != nil,
		})
	case "write_to_clipboard_text":
		text, exists := params.Arguments["text_to_copy"].(string)
		if !exists {
			writeResponse(id, CallResult{
				Content: []ToolContent{},
				IsError: true,
			})
			return
		}
		writeToClipboardText(text)
		writeResponse(id, CallResult{
			Content: []ToolContent{},
			IsError: false,
		})
	case "read_clipboard_image":
		contentArray, err := readClipboardImage()
		writeResponse(id, CallResult{
			Content: contentArray,
			IsError: err != nil,
		},
		)
	case "write_to_clipboard_image":
		base64png, exists := params.Arguments["base64_png"].(string)
		if !exists {
			writeResponse(id, CallResult{
				Content: []ToolContent{},
				IsError: true,
			})
			return
		}
		writeToClipboardImage(base64png)
		writeResponse(id, CallResult{
			Content: []ToolContent{},
			IsError: false,
		})
	default:
		writeResponse(id, CallResult{
			Content: []ToolContent{},
			IsError: true,
		})
	}
}

func writeToClipboardImage(base64png string) error {
	imgBytes, err := base64.StdEncoding.DecodeString(base64png)
	if err != nil {
		return fmt.Errorf("Invalid base64 image data: %v", err)
	}
	clipboard.Write(clipboard.FmtImage, imgBytes)
	return nil
}

func readClipboardImage() ([]ToolContent, error) {
	imgBytes := clipboard.Read(clipboard.FmtImage)
	if len(imgBytes) == 0 {
		return []ToolContent{}, fmt.Errorf("clipboard is empty")
	}
	base64Str := base64.StdEncoding.EncodeToString(imgBytes)
	return []ToolContent{
		{
			Type:     "image",
			Data:     base64Str,
			MimeType: "image/png",
		},
		{
			Type: "text",
			Text: "Read image successfully from clipboard",
		},
	}, nil
}

func writeToClipboardText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}

func readClipboardText() (string, error) {
	textBytes := clipboard.Read(clipboard.FmtText)
	if len(textBytes) == 0 {
		return "", fmt.Errorf("clipboard is empty")
	}
	return string(textBytes), nil
}

func writeResponse(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	bytes, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Failed to marshal response: %v", err)
		return
	}
	fmt.Printf("%s\n", string(bytes))
}

func handleInitialize(id interface{}) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "Clipboard-MCP",
			Version: "1.0.0",
		},
		Capabilities: ServerCapabilities{
			Tools: map[string]interface{}{},
		},
	}
	writeResponse(id, result)
	logger.Printf("Send handshake")
}
