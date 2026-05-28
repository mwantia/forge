package template

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// WithAnyValue registers a variable from an arbitrary Go value, converting it
// to the equivalent cty type. map[string]any becomes an object, []any a tuple,
// and primitives map directly. Fields are accessible as ${name.field} or
// {{ .name.field }}.
func WithAnyValue(name string, val any) TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		cv, err := anyToCty(val)
		if err != nil {
			return fmt.Errorf("template: WithAnyValue(%q): %w", name, err)
		}
		t.vars[name] = cv
		return nil
	}
}

// WithJsonValue registers a variable by unmarshalling raw JSON bytes and
// converting the result to cty. Object fields are accessible as ${name.field}
// or {{ .name.field }}.
func WithJsonValue(name string, data []byte) TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("template: WithJsonValue(%q): %w", name, err)
		}

		cv, err := anyToCty(v)
		if err != nil {
			return fmt.Errorf("template: WithJsonValue(%q): %w", name, err)
		}
		t.vars[name] = cv
		return nil
	}
}

func anyToCty(v any) (cty.Value, error) {
	switch val := v.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType), nil
	case string:
		return cty.StringVal(val), nil
	case bool:
		return cty.BoolVal(val), nil
	case float64:
		return cty.NumberFloatVal(val), nil
	case int:
		return cty.NumberIntVal(int64(val)), nil
	case int64:
		return cty.NumberIntVal(val), nil
	case map[string]any:
		if len(val) == 0 {
			return cty.EmptyObjectVal, nil
		}
		attrs := make(map[string]cty.Value, len(val))
		for k, item := range val {
			cv, err := anyToCty(item)
			if err != nil {
				return cty.NilVal, fmt.Errorf("%s: %w", k, err)
			}
			attrs[k] = cv
		}
		return cty.ObjectVal(attrs), nil
	case []any:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType), nil
		}
		elems := make([]cty.Value, len(val))
		for i, item := range val {
			cv, err := anyToCty(item)
			if err != nil {
				return cty.NilVal, fmt.Errorf("[%d]: %w", i, err)
			}
			elems[i] = cv
		}
		return cty.TupleVal(elems), nil
	default:
		return cty.NilVal, fmt.Errorf("unsupported type %T", v)
	}
}

// WithValue registers a single variable under the given dot-notated name.
// Dot segments are expanded into nested cty objects at render/eval time, so
// "session.id" becomes accessible as ${session.id} in templates.
//
//	${name}         — top-level variable  (name = "name")
//	${ns.field}     — nested field        (name = "ns.field")
func WithValue(name string, val cty.Value) TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.vars[name] = val

		return nil
	}
}

// WithRuntime registers read-only variables describing the current process and
// host. All values are captured once at the time this option is applied.
//
// Process:
//
//	${runtime.version}            — Go runtime version (e.g. "go1.22.3")
//	${runtime.pid}                — process ID
//	${runtime.uid}                — effective user ID
//	${runtime.cwd}                — working directory at startup
//
// Host node:
//
//	${runtime.node.hostname}      — machine hostname
//	${runtime.node.arch}          — CPU architecture (e.g. "amd64", "arm64")
//	${runtime.node.cpu_count}     — number of logical CPUs
//	${runtime.node.ipv4}          — primary outbound IPv4 address
//	${runtime.node.os.name}       — OS identifier (e.g. "linux", "darwin")
//	${runtime.node.os.version}    — OS pretty-name from /etc/os-release
func WithRuntime() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.vars["runtime.version"] = cty.StringVal(runtime.Version())
		t.vars["runtime.pid"] = cty.NumberIntVal(int64(os.Getpid()))
		t.vars["runtime.uid"] = cty.NumberIntVal(int64(os.Getuid()))
		t.vars["runtime.cwd"] = cty.StringVal(getRuntimeCWD())

		t.vars["runtime.node.hostname"] = cty.StringVal(getRuntimeHostname())
		t.vars["runtime.node.arch"] = cty.StringVal(runtime.GOARCH)
		t.vars["runtime.node.cpu_count"] = cty.NumberIntVal(int64(runtime.NumCPU()))
		t.vars["runtime.node.ipv4"] = cty.StringVal(getRuntimePrimaryIPv4())

		t.vars["runtime.node.os.name"] = cty.StringVal(runtime.GOOS)
		t.vars["runtime.node.os.version"] = cty.StringVal(getRuntimeOSRelease())

		return nil
	}
}

func getRuntimeCWD() string {
	cwd, _ := os.Getwd()
	return cwd
}

func getRuntimeHostname() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return hostname
}

func getRuntimePrimaryIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "127.0.0.1"
	}
	return addr.IP.String()
}

// getRuntimeOSRelease reads PRETTY_NAME from /etc/os-release (Linux/macOS).
// Falls back to runtime.GOOS when the file is absent or unparseable.
func getRuntimeOSRelease() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), "=")
		if ok && k == "PRETTY_NAME" {
			return strings.Trim(v, `"`)
		}
	}
	return runtime.GOOS
}
