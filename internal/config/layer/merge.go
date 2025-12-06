package layer

import "strings"

// DeepMerge recursively merges src into dst.
// Values in src override values in dst.
// Maps are merged recursively; other types are replaced.
func DeepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	if src == nil {
		return dst
	}

	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = cloneValue(srcVal)
			continue
		}

		// If both are maps, merge recursively
		srcMap, srcIsMap := srcVal.(map[string]any)
		dstMap, dstIsMap := dstVal.(map[string]any)
		if srcIsMap && dstIsMap {
			dst[key] = DeepMerge(dstMap, srcMap)
		} else {
			// Otherwise, src replaces dst
			dst[key] = cloneValue(srcVal)
		}
	}

	return dst
}

// cloneValue creates a deep copy of a value.
func cloneValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		return cloneSlice(v)
	default:
		return val
	}
}

// GetByPath retrieves a value from a nested map using a dot-separated path.
func GetByPath(data map[string]any, path string) (any, bool) {
	if data == nil {
		return nil, false
	}

	parts := strings.Split(path, ".")
	current := any(data)

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		val, exists := m[part]
		if !exists {
			return nil, false
		}

		current = val
	}

	return current, true
}

// SetByPath sets a value in a nested map using a dot-separated path.
// Creates intermediate maps as needed.
func SetByPath(data map[string]any, path string, value any) {
	if data == nil {
		return
	}

	parts := strings.Split(path, ".")
	current := data

	// Navigate/create intermediate maps
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			// Create intermediate map
			next := make(map[string]any)
			current[part] = next
			current = next
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}

// DeleteByPath removes a value from a nested map using a dot-separated path.
// Returns true if the value was found and deleted.
func DeleteByPath(data map[string]any, path string) bool {
	if data == nil {
		return false
	}

	parts := strings.Split(path, ".")
	current := data

	// Navigate to the parent of the target
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}

	// Delete the target
	key := parts[len(parts)-1]
	if _, exists := current[key]; exists {
		delete(current, key)
		return true
	}

	return false
}

// FlattenMap flattens a nested map into a single-level map with dot-separated keys.
func FlattenMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	flattenMapRecursive(data, "", result)
	return result
}

func flattenMapRecursive(data map[string]any, prefix string, result map[string]any) {
	for key, val := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if nested, ok := val.(map[string]any); ok {
			flattenMapRecursive(nested, fullKey, result)
		} else {
			result[fullKey] = val
		}
	}
}

// UnflattenMap converts a flattened map with dot-separated keys back to nested structure.
func UnflattenMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for path, val := range data {
		SetByPath(result, path, val)
	}
	return result
}

// DiffMaps returns the paths that differ between two maps.
// Returns added, modified, and removed paths.
func DiffMaps(old, new map[string]any) (added, modified, removed []string) {
	oldFlat := FlattenMap(old)
	newFlat := FlattenMap(new)

	// Find added and modified
	for path, newVal := range newFlat {
		if oldVal, exists := oldFlat[path]; exists {
			if !valuesEqual(oldVal, newVal) {
				modified = append(modified, path)
			}
		} else {
			added = append(added, path)
		}
	}

	// Find removed
	for path := range oldFlat {
		if _, exists := newFlat[path]; !exists {
			removed = append(removed, path)
		}
	}

	return added, modified, removed
}

// valuesEqual compares two values for equality.
func valuesEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare by type
	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}
		return mapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok {
			return false
		}
		return slicesEqual(va, vb)
	default:
		return a == b
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !valuesEqual(va, vb) {
			return false
		}
	}
	return true
}

func slicesEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !valuesEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
