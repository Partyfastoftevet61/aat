package protoreg

import "strings"

// SplitFullMethod splits "pkg.Service/Method", the form gRPC uses on the wire,
// and also accepts "pkg.Service.Method", the form protobuf uses for a full
// name. A leading slash, as a wire method carries, is ignored.
//
// It lives here because a graph's proto: and a template's rpc: must read a
// reference the same way, and the two are parsed in packages that cannot
// import each other. Registry.Method takes the halves this returns.
func SplitFullMethod(ref string) (service, method string, ok bool) {
	ref = strings.TrimPrefix(ref, "/")
	if i := strings.LastIndex(ref, "/"); i > 0 {
		service, method = ref[:i], ref[i+1:]
	} else if i := strings.LastIndex(ref, "."); i > 0 {
		service, method = ref[:i], ref[i+1:]
	} else {
		return "", "", false
	}
	if service == "" || method == "" || strings.Contains(method, "/") {
		return "", "", false
	}
	return service, method, true
}
