package runtime

import (
	"fmt"

	opgo_poc2 "github.com/rrgmc/opgo-poc2"
	"github.com/rrgmc/opgo-poc2/opgocore"
	"github.com/rrgmc/opgo-poc2/opgotypeslice"
)

var opgoConfig = opgo_poc2.NewConfig(
	opgo_poc2.WithCastIntsAndFloats(true),
	opgo_poc2.WithDefaultIntValue(func(value int64) opgo_poc2.Type {
		return opgo_poc2.IntValue(value)
	}),
	opgo_poc2.WithDefaultUintValue(func(value uint64) opgo_poc2.Type {
		return opgo_poc2.UintValue(value)
	}),
	opgo_poc2.WithDefaultFloatValue(func(value float64) opgo_poc2.Type {
		return opgo_poc2.Float64Value(value)
	}),
	opgo_poc2.WithTypeFactory(opgotypeslice.NewPrimitiveTypeFactory()),
	// opgo_poc2.WithTypeFactory(opgotype.NewPrimitiveReflectTypeFactory()),
	opgo_poc2.WithTypeFactory(opgocore.TypeFactoryFunc(func(value any, options opgocore.Options) (opgocore.Type, bool) {
		return opgo_poc2.ValueDeepEqual{value}, true
	})),
)

func Equal(a, b interface{}) bool {
	v, err := opgoConfig.EqualsCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func Less(a, b interface{}) bool {
	v, err := opgoConfig.LessCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func More(a, b interface{}) bool {
	v, err := opgoConfig.GreaterCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func LessOrEqual(a, b interface{}) bool {
	v, err := opgoConfig.LessEqCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func MoreOrEqual(a, b interface{}) bool {
	v, err := opgoConfig.GreaterEqCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func retType(value opgo_poc2.Type) any {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	return value.Value()
}

func retTypeFloat(value opgo_poc2.Type) float64 {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	vf, err := opgoConfig.TypeToFloat(value)
	if err != nil {
		panic(fmt.Sprintf("error converting type '%s' to float: %s", value.TypeName(), err))
	}
	return vf
}

func retTypeInt(value opgo_poc2.Type) int {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	vf, err := opgoConfig.TypeToInt(value)
	if err != nil {
		panic(fmt.Sprintf("error converting type '%s' to int: %s", value.TypeName(), err))
	}
	return int(vf)
}

func Add(a, b interface{}) interface{} {
	return retType(opgoConfig.Add(a, b))
}

func Subtract(a, b interface{}) interface{} {
	return retType(opgoConfig.Subtract(a, b))
}

func Multiply(a, b interface{}) interface{} {
	return retType(opgoConfig.Multiply(a, b))
}

func Divide(a, b interface{}) float64 {
	return retTypeFloat(opgoConfig.DivideFloat(a, b))
}

func Modulo(a, b interface{}) int {
	return retTypeInt(opgoConfig.Modulo(a, b))
}
