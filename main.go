package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ysravankumarreddy/mcp-protocol"
	"golang.org/x/sys/windows/registry"
)

var logger log.Logger

type RegistryResult struct {
	SubKeys []string               `json:"sub_keys,omitempty"`
	Values  map[string]interface{} `json:"values,omitempty"`
}

func init() {
	logger = *log.New(os.Stderr, "[Registry-MCP]", log.Ltime)
}

func main() {
	logger.Println("Starting Registry-MCP...")

	decoder := json.NewDecoder(os.Stdin)
	for {
		var req mcp.Request
		if err := decoder.Decode(&req); err != nil {
			logger.Printf("Error decoding request: %v\n", err)
			break
		}
		dispatchRequest(req)
	}
}

func dispatchRequest(req mcp.Request) {
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
	var params mcp.CallParams
	if err := json.Unmarshal(rawMessage, &params); err != nil {
		logger.Printf("Failed to unmarshal call params: %v", err)
		return
	}
	var result []mcp.ToolContent
	var executionErr error

	switch params.Name {
	case "read_registry":
		hive, okHive := params.Arguments["hive"].(string)
		path, okPath := params.Arguments["path"].(string)
		if !okHive || !okPath {
			executionErr = fmt.Errorf("Invalid arguments for read_registry")
			break
		}
		regData, err := readRegistry(hive, path)
		if err != nil {
			executionErr = fmt.Errorf("registry read failed: %v", err)
			break
		}
		jsonData, err := json.MarshalIndent(regData, "", "  ")
		result = append(result, mcp.ToolContent{
			Type: "text",
			Text: string(jsonData),
		})
	default:
		executionErr = fmt.Errorf("unknown tool: %s", params.Name)
	}
	sendCallResult(id, result, executionErr)
}

func sendCallResult(id interface{}, content []mcp.ToolContent, executionErr error) {
	result := mcp.CallResult{
		Content: content,
		IsError: executionErr != nil,
	}

	if executionErr != nil {
		result.Content = []mcp.ToolContent{
			{
				Type: "text",
				Text: fmt.Sprintf("Error executing tool: %v", executionErr),
			},
		}
	}
	writeResponse(id, result)
}

func handleToolList(id interface{}) {
	result := mcp.ToolListResult{
		Tools: []mcp.Tool{
			{
				Name:        "read_registry",
				Description: "Reads a registry key and returns its subkeys and values",
				InputSchema: mcp.InputSchema{
					Type: "object",
					Properties: map[string]mcp.PropertySchema{
						"hive": {
							Type:        "string",
							Description: "Registry hive (e.g., HKLM, HKCU)",
						},
						"path": {
							Type:        "string",
							Description: "Registry key path (e.g., Software\\Microsoft)",
						},
					},
					Required: []string{"hive", "path"},
				},
			},
		},
	}
	writeResponse(id, result)
}

func handleInitialize(id interface{}) {
	result := mcp.InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: mcp.ServerInfo{
			Name:    "Registry-MCP",
			Version: "1.0.0",
		},
		Capabilities: mcp.ServerCapabilities{
			Tools: map[string]interface{}{},
		},
	}
	writeResponse(id, result)
	logger.Printf("Send handshake")
}

func getHive(key string) (hive registry.Key, e error) {
	switch strings.ToUpper(key) {
	case "HKLM", "HKEY_LOCAL_MACHINE":
		return registry.LOCAL_MACHINE, nil
	case "HKCU", "HKEY_CURRENT_USER":
		return registry.CURRENT_USER, nil
	case "HKCR", "HKEY_CLASSES_ROOT":
		return registry.CLASSES_ROOT, nil
	case "HKU", "HKEY_USERS":
		return registry.USERS, nil
	case "HKCC", "HKEY_CURRENT_CONFIG":
		return registry.CURRENT_CONFIG, nil
	case "HKPD", "HKEY_PERFORMANCE_DATA":
		return registry.PERFORMANCE_DATA, nil
	default:
		return 0, fmt.Errorf("unknown registry hive: %s", key)
	}
}

func readRegistry(hiveStr, path string) (RegistryResult, error) {
	rootKey, err := getHive(hiveStr)
	if err != nil {
		return RegistryResult{}, err
	}
	key, err := registry.OpenKey(rootKey, path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return RegistryResult{}, fmt.Errorf("Failed to read key: %v", err)
	}
	defer key.Close()

	result := RegistryResult{
		Values: make(map[string]interface{}),
	}

	subKeys, err := key.ReadSubKeyNames(-1)
	if err == nil {
		result.SubKeys = subKeys
	}
	valueNames, err := key.ReadValueNames(-1)
	if err != nil {
		return result, nil // Return subkeys even if values can't be read
	}
	for _, name := range valueNames {
		val, _, err := key.GetStringValue(name)
		if err == nil {
			result.Values[name] = val
			continue
		}

		intVal, _, err := key.GetIntegerValue(name)
		if err == nil {
			result.Values[name] = intVal
			continue
		}

		result.Values[name] = "Unsupported value type"
	}

	return result, nil
}

func writeResponse(id interface{}, result interface{}) {
	resp := mcp.Response{
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
