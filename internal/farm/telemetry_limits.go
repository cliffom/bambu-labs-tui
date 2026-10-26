package farm

// Bounds apply independently to each decoded envelope and retained print
// snapshot. Unknown fields remain supported within these limits. Key/string
// bytes plus the node count bound memory; depth and collection sizes bound
// recursive work and the identity-array merge's quadratic search.
const (
	maxTelemetryBytes      = 2 * 1024 * 1024
	maxTelemetryNodes      = 8192
	maxTelemetryDepth      = 32
	maxTelemetryCollection = 256
)

func telemetryWithinLimits(value any) bool {
	nodes, bytes := maxTelemetryNodes, maxTelemetryBytes
	return checkTelemetry(value, 0, &nodes, &bytes)
}

func checkTelemetry(value any, depth int, nodes, bytes *int) bool {
	*nodes--
	if depth > maxTelemetryDepth || *nodes < 0 || *bytes < 0 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		if len(v) > maxTelemetryCollection {
			return false
		}
		for key, child := range v {
			*bytes -= len(key)
			if !checkTelemetry(child, depth+1, nodes, bytes) {
				return false
			}
		}
	case []any:
		if len(v) > maxTelemetryCollection {
			return false
		}
		for _, child := range v {
			if !checkTelemetry(child, depth+1, nodes, bytes) {
				return false
			}
		}
	case string:
		*bytes -= len(v)
	}
	return *bytes >= 0
}

// cloneTelemetry only receives previously accepted, bounded JSON trees.
func cloneTelemetry(value any) any {
	switch v := value.(type) {
	case map[string]any:
		copy := make(map[string]any, len(v))
		for key, child := range v {
			copy[key] = cloneTelemetry(child)
		}
		return copy
	case []any:
		copy := make([]any, len(v))
		for i, child := range v {
			copy[i] = cloneTelemetry(child)
		}
		return copy
	default:
		return value
	}
}
