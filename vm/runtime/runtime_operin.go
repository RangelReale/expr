package runtime

import (
	"fmt"

	operin_poc1 "github.com/rrgmc/operin-poc1"
	"github.com/rrgmc/operin-poc1/operincore"
	"github.com/rrgmc/operin-poc1/operintypeslice"
)

var operinConfig = operin_poc1.NewConfig(
	operin_poc1.WithCastIntsAndFloats(true),
	operin_poc1.WithDefaultIntValue(func(value int64) operin_poc1.Type {
		return operin_poc1.IntValue(value)
	}),
	operin_poc1.WithDefaultUintValue(func(value uint64) operin_poc1.Type {
		return operin_poc1.UintValue(value)
	}),
	operin_poc1.WithDefaultFloatValue(func(value float64) operin_poc1.Type {
		return operin_poc1.Float64Value(value)
	}),
	operin_poc1.WithTypeFactory(operintypeslice.NewPrimitiveTypeFactory()),
	// operin_poc1.WithTypeFactory(operintype.NewPrimitiveReflectTypeFactory()),
	operin_poc1.WithTypeFactory(operincore.TypeFactoryFunc(func(value any, options operincore.Options) (operincore.Type, bool) {
		return operin_poc1.ValueDeepEqual{value}, true
	})),
)

func Equal(a, b interface{}) bool {
	v, err := operinConfig.EqualsCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func Less(a, b interface{}) bool {
	v, err := operinConfig.LessCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func More(a, b interface{}) bool {
	v, err := operinConfig.GreaterCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func LessOrEqual(a, b interface{}) bool {
	v, err := operinConfig.LessEqCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func MoreOrEqual(a, b interface{}) bool {
	v, err := operinConfig.GreaterEqCheck(a, b)
	if err != nil {
		panic(err)
	}
	return v
}

func retType(value operin_poc1.Type) any {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	return value.Value()
}

func retTypeFloat(value operin_poc1.Type) float64 {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	vf, err := operinConfig.TypeToFloat(value)
	if err != nil {
		panic(fmt.Sprintf("error converting type '%s' to float: %s", value.TypeName(), err))
	}
	return vf
}

func retTypeInt(value operin_poc1.Type) int {
	if !value.IsValueValid() {
		panic(fmt.Sprintf("type '%s' has no valid value", value.TypeName()))
	}
	vf, err := operinConfig.TypeToInt(value)
	if err != nil {
		panic(fmt.Sprintf("error converting type '%s' to int: %s", value.TypeName(), err))
	}
	return int(vf)
}

func Add(a, b interface{}) interface{} {
	return retType(operinConfig.Add(a, b))
}

func Subtract(a, b interface{}) interface{} {
	return retType(operinConfig.Subtract(a, b))
}

func Multiply(a, b interface{}) interface{} {
	return retType(operinConfig.Multiply(a, b))
}

func Divide(a, b interface{}) float64 {
	return retTypeFloat(operinConfig.DivideFloat(a, b))
}

func Modulo(a, b interface{}) int {
	return retTypeInt(operinConfig.Modulo(a, b))
}
