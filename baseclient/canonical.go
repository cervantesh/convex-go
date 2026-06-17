package baseclient

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cervantesh/convex-go/internal/core"
)

func canonicalQuery(path string, args any) (string, []Value, string, error) {
	canonicalPath, err := canonicalQueryPath(path)
	if err != nil {
		return "", nil, "", err
	}
	encodedArg, err := syncQueryArgValue(args)
	if err != nil {
		return "", nil, "", err
	}
	encodedArgs := []Value{encodedArg}
	wireArgs := make([]any, len(encodedArgs))
	for i, arg := range encodedArgs {
		wire, err := arg.ConvexJSON()
		if err != nil {
			return "", nil, "", err
		}
		wireArgs[i] = wire
	}
	tokenJSON, err := stableJSON(map[string]any{
		"udfPath": canonicalPath,
		"args":    wireArgs,
	})
	if err != nil {
		return "", nil, "", err
	}
	return canonicalPath, encodedArgs, tokenJSON, nil
}

func canonicalRequest(path string, args any) (string, []Value, error) {
	canonicalPath, err := canonicalQueryPath(path)
	if err != nil {
		return "", nil, err
	}
	encodedArg, err := syncQueryArgValue(args)
	if err != nil {
		return "", nil, err
	}
	return canonicalPath, []Value{encodedArg}, nil
}

func canonicalQueryPath(path string) (string, error) {
	normalized, err := ParseFunctionPath(path)
	if err != nil {
		return "", err
	}
	parsed, err := ParseUDFPath(normalized)
	if err != nil {
		return "", err
	}
	return syncWireUDFPath(parsed.Canonicalize()), nil
}

func syncWireUDFPath(path CanonicalizedUDFPath) string {
	module := strings.TrimSuffix(path.Module.String(), ".js")
	return module + ":" + string(path.Function)
}

func syncQueryArgValue(args any) (Value, error) {
	encodedArgs, err := encodeArgs(args)
	if err != nil {
		return Value{}, err
	}
	return valueFromWire(encodedArgs)
}

func encodeArgs(args any) (map[string]any, error) {
	if args == nil {
		return map[string]any{}, nil
	}
	encoded, err := core.EncodeValue(args)
	if err != nil {
		return nil, err
	}
	obj, ok := encoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("convex: function arguments must encode to an object, got %T", encoded)
	}
	return obj, nil
}

func valueFromWire(value any) (Value, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Value{}, err
	}
	return core.ParseValueJSON(data)
}

func stableJSON(value any) (string, error) {
	normalized, err := stableJSONValue(value)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func stableJSONValue(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(v))
		for _, key := range keys {
			child, err := stableJSONValue(v[key])
			if err != nil {
				return nil, err
			}
			out[key] = child
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			child, err := stableJSONValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = child
		}
		return out, nil
	case nil, bool, string, float64:
		return v, nil
	default:
		return nil, fmt.Errorf("convex: unsupported stable JSON token value %T", value)
	}
}
