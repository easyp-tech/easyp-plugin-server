// Package internal provide helpers for database.
package internal

import (
	"reflect"
	"runtime"
	"strings"
)

// MethodsOf require pointer to interface (e.g.: new(YourInterface)) and
// returns all it methods.
func MethodsOf(v any) []string {
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Interface {
		panic("require pointer to interface")
	}
	typ = typ.Elem()
	methods := make([]string, typ.NumMethod())
	for i := range typ.NumMethod() {
		methods[i] = typ.Method(i).Name
	}

	return methods
}

// CallerMethodName returns caller's method name for given stack depth.
func CallerMethodName(skip int) string {
	return methodName(callerName(1 + skip))
}

// Returns:
//
//	[example.com/path/]{dir|"main"}.{func|type.method}[."func"id[.id]...]
func callerName(skip int) string {
	pc, _, _, _ := runtime.Caller(1 + skip) //nolint:dogsled // usage of 3 blanks is intended

	return runtime.FuncForPC(pc).Name()
}

// Returns type.method or func."func"id if it's not a method.
func typeMethodName(name string) string {
	start := strings.LastIndexByte(name, '/') + 1
	pos := strings.IndexByte(name[start:], '.')
	if pos == -1 {
		panic("bad name: " + name)
	}
	start += pos + 1
	pos = strings.IndexByte(name[start:], '.')
	if pos == -1 {
		panic("not a method name: " + name)
	}
	end := strings.IndexByte(name[start+pos+1:], '.')
	if end == -1 {
		end = len(name)
	} else {
		end += start + pos + 1
	}

	return name[start:end]
}

// Returns method or "func"id if it's not a method.
func methodName(name string) string {
	name = typeMethodName(name)
	_, after, found := strings.Cut(name, ".")
	if !found {
		panic("not a method name: " + name)
	}

	return after
}

// Returns func or method if it's not a function.
