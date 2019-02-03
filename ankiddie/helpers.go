package ankiddie

import (
	"context"
	"reflect"
)

func ankoStrengthen(ctx context.Context, fn interface{}, argsForTypes []interface{}) interface{} {
	// fn is the original anko function
	fType := reflect.TypeOf(fn)
	if fType == nil || fType.Kind() != reflect.Func {
		return fn
	}

	ins := make([]reflect.Type, 0)
	outs := make([]reflect.Type, 0)

	i := 0
	transformReturn := false
	// anko functions now take a context as the first argument, hence fType.NumIn()-1
	for ; i < fType.NumIn()-1 && i < len(argsForTypes); i++ {
		if argsForTypes[i] == nil {
			break
		}
		ins = append(ins, reflect.TypeOf(argsForTypes[i]))
	}

	if i < len(argsForTypes) && argsForTypes[i] == nil {
		transformReturn = true
		i++
	}

	if transformReturn {
		for ; i < len(argsForTypes); i++ {
			outs = append(outs, reflect.TypeOf(argsForTypes[i]))
		}
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
		result := reflect.ValueOf(fn).Call(args)
		// we must also convert the result, because all anko functions always return (reflect.Value, error)
		retVal := result[0].Interface().(reflect.Value)
		k := retVal.Kind()
		switch k {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr:
			if retVal.IsNil() {
				return []reflect.Value{}
			}
		}
		converted := make([]reflect.Value, outsCount)
		retIfaces, ok := retVal.Interface().([]interface{})
		if !ok {
			return converted
		}
		for i := 0; i < outsCount && i < len(retIfaces); i++ {
			converted[i] = reflect.ValueOf(retIfaces[i])
		}
		return converted
	})
	return transformedFunc.Interface()
}
