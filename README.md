### Installation

1. Download the latest `registry-mcp.exe` from the [Releases page](https://github.com/ysravankumarreddy/registry-mcp/releases)
2. Save it to a permanent location (e.g., `C:\mcp\registry-mcp.exe`).
3. Add the following to your Claude Desktop config (`%APPDATA%\Claude\claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "registry-mcp": {
      "command": "c:\mcp\registry-mcp.exe"
    }
  }
} 
