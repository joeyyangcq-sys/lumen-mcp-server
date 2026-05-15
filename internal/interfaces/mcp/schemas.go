package mcp

import "encoding/json"

func inputSchemaFor(toolName string) json.RawMessage {
	switch toolName {
	case "list_routes", "list_services", "list_upstreams", "list_plugin_configs", "list_global_rules":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"page":      {"type": "integer", "description": "Page number"},
				"page_size": {"type": "integer", "description": "Items per page"},
				"keyword":   {"type": "string",  "description": "Search keyword"}
			}
		}`)

	case "get_route", "get_service", "get_upstream", "get_plugin_config", "get_global_rule":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "Resource ID"}
			},
			"required": ["id"]
		}`)

	case "put_route", "put_service", "put_upstream", "put_plugin_config", "put_global_rule",
		"patch_route", "patch_service", "patch_upstream", "patch_plugin_config", "patch_global_rule":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"id":   {"type": "string", "description": "Resource ID"},
				"body": {"type": "object", "description": "Resource body"}
			},
			"required": ["id", "body"]
		}`)

	case "delete_route", "delete_service", "delete_upstream", "delete_plugin_config", "delete_global_rule":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "Resource ID"}
			},
			"required": ["id"]
		}`)

	case "preview_import", "apply_import":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"request": {"type": "object", "description": "Import bundle payload"}
			}
		}`)

	case "export_bundle":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"format": {"type": "string",               "description": "Export format"},
				"kinds":  {"type": "array", "items": {"type": "string"}, "description": "Resource kinds to export"}
			}
		}`)

	case "history_list":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"limit": {"type": "integer", "description": "Max entries to return"}
			}
		}`)

	case "history_rollback":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "History entry ID to rollback to"}
			},
			"required": ["id"]
		}`)

	case "analyze_latency":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"route_id":            {"type": "string",  "description": "Optional route ID filter"},
				"upstream_id":         {"type": "string",  "description": "Optional upstream ID filter"},
				"min_samples":         {"type": "integer", "description": "Ignore series with fewer samples"},
				"quantile":            {"type": "number",  "description": "Quantile used for recommendations, e.g. 0.95"},
				"safety_factor":       {"type": "number",  "description": "Multiplier over observed latency when recommending timeout"},
				"include_non_2xx":     {"type": "boolean", "description": "Whether to include non-2xx status classes"},
				"min_connect_ms":      {"type": "number",  "description": "Minimum connect timeout in milliseconds"},
				"min_read_ms":         {"type": "number",  "description": "Minimum read/send timeout in milliseconds"},
				"max_connect_seconds": {"type": "integer", "description": "Upper bound for connect timeout recommendation"},
				"max_read_seconds":    {"type": "integer", "description": "Upper bound for read/send timeout recommendation"}
			}
		}`)

	case "tune_upstream_timeout":
		return json.RawMessage(`{
			"type": "object",
			"properties": {
				"route_id":            {"type": "string",  "description": "Optional route ID filter"},
				"upstream_id":         {"type": "string",  "description": "Optional upstream ID filter"},
				"dry_run":             {"type": "boolean", "description": "If true, only return recommendation without update"},
				"min_samples":         {"type": "integer", "description": "Ignore series with fewer samples"},
				"quantile":            {"type": "number",  "description": "Quantile used for recommendations, e.g. 0.95"},
				"safety_factor":       {"type": "number",  "description": "Multiplier over observed latency when recommending timeout"},
				"include_non_2xx":     {"type": "boolean", "description": "Whether to include non-2xx status classes"},
				"min_connect_ms":      {"type": "number",  "description": "Minimum connect timeout in milliseconds"},
				"min_read_ms":         {"type": "number",  "description": "Minimum read/send timeout in milliseconds"},
				"max_connect_seconds": {"type": "integer", "description": "Upper bound for connect timeout recommendation"},
				"max_read_seconds":    {"type": "integer", "description": "Upper bound for read/send timeout recommendation"}
			}
		}`)

	case "get_schema", "list_plugins", "get_stats":
		return json.RawMessage(`{"type": "object"}`)

	default:
		return json.RawMessage(`{"type": "object"}`)
	}
}
