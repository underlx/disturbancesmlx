package ankiddie

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gbl08ma/anko/vm"
)

var reflectValueType = reflect.TypeOf(reflect.Value{})
var errorType = reflect.ValueOf([]error{nil}).Index(0).Type()
var vmErrorType = reflect.TypeOf(&vm.Error{})

func ankoStrengthen(ctx context.Context, fn interface{}, argsForTypes []interface{}) interface{} {
	types := make([]reflect.Type, len(argsForTypes))
	for i, arg := range argsForTypes {
		types[i] = reflect.TypeOf(arg)
		if s, ok := argsForTypes[i].(string); ok && s == "error" {
			types[i] = errorType
		}
	}
	return ankoStrengthenWithTypes(ctx, fn, types)
}

func ankoStrengthenWithTypes(ctx context.Context, fn interface{}, argTypes []reflect.Type) interface{} {
	// fn is the original anko function
	fType := reflect.TypeOf(fn)
	if fType == nil || fType.Kind() != reflect.Func {
		return fn
	}

	ins := make([]reflect.Type, 0)
	outs := make([]reflect.Type, 0)

	i := 0

	// anko functions now take a context as the first argument, hence fType.NumIn()-1
	for ; i < fType.NumIn()-1 && i < len(argTypes); i++ {
		if argTypes[i] == nil {
			break
		}
		t := argTypes[i]
		if t.Implements(errorType) {
			t = errorType
		}
		ins = append(ins, t)
	}

	if i < len(argTypes) && argTypes[i] == nil {
		i++
	}

	for ; i < len(argTypes); i++ {
		t := argTypes[i]
		if t.Implements(errorType) {
			t = errorType
		}
		outs = append(outs, t)
	}

	outsCount := len(outs)
	variadic := fType.IsVariadic()
	funcType := reflect.FuncOf(ins, outs, variadic)
	transformedFunc := reflect.MakeFunc(funcType, func(in []reflect.Value) []reflect.Value {
		args := make([]reflect.Value, len(in)+1)
		args[0] = reflect.ValueOf(ctx)
		for i, arg := range in {
			// functions in anko always appear to golang as if all their arguments were reflect.Values
			// if we don't wrap args like this, Call below complains that e.g.
			// "panic: reflect: Call using *discordgo.Session as type reflect.Value"
			args[i+1] = reflect.ValueOf(arg)
		}
		rvs := reflect.ValueOf(fn).Call(args)
		if len(rvs) != 2 {
			panic(fmt.Sprintf("strengthen: function did not return 2 values but returned %v values", len(rvs)))
		}
		if rvs[0].Type() != reflectValueType {
			panic(fmt.Sprintf("strengthen: function value 1 did not return reflect value type but returned %v type", rvs[0].Type().String()))
		}
		if rvs[1].Type() != reflectValueType {
			panic(fmt.Sprintf("strengthen: function value 2 did not return reflect value type but returned %v type", rvs[1].Type().String()))
		}
		// we must also convert the result, because all anko functions always return (reflect.Value, error)
		conv := func(value reflect.Value, t reflect.Type) reflect.Value {
			if (value.Kind() == reflect.Chan ||
				value.Kind() == reflect.Func ||
				value.Kind() == reflect.Map ||
				value.Kind() == reflect.Ptr ||
				value.Kind() == reflect.Interface ||
				value.Kind() == reflect.Slice) &&
				value.IsNil() {
				return reflect.Zero(t)
			}
			return value.Convert(t)
		}
		convFromSliceItem := func(value reflect.Value, t reflect.Type) reflect.Value {
			if err, isError := value.Interface().(error); isError {
				return reflect.ValueOf(&err).Elem()
			}
			if (value.Kind() == reflect.Chan ||
				value.Kind() == reflect.Func ||
				value.Kind() == reflect.Map ||
				value.Kind() == reflect.Ptr ||
				value.Kind() == reflect.Interface ||
				value.Kind() == reflect.Slice) &&
				value.IsNil() {
				return reflect.Zero(t)
			}
			if rv, isReflect := value.Interface().(reflect.Value); isReflect {
				return rv
			}
			return reflect.ValueOf(value.Interface())
		}
		rv := rvs[0].Interface().(reflect.Value)
		rvError := rvs[1].Interface().(reflect.Value)
		if rv.Kind() == reflect.Slice {
			converted := make([]reflect.Value, outsCount)
			for i := 0; i < outsCount && i < rv.Len(); i++ {
				converted[i] = convFromSliceItem(rv.Index(i), outs[i])
			}
			return converted
		}
		if outsCount == 2 {
			return []reflect.Value{conv(rv, outs[0]), conv(rvError, outs[1])}
		}
		if outsCount > 0 {
			return []reflect.Value{conv(rv, outs[0])}
		}
		return []reflect.Value{}
	})
	return transformedFunc.Interface()
}
